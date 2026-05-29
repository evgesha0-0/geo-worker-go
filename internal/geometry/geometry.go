package geometry

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"geo-worker-go/internal/models"
	"github.com/peterstace/simplefeatures/geom"
)

type GeoTile struct {
	X int
	Y int
	Z int
}

const (
	fullCircleDegrees      = 360.0
	halfCircleDegrees      = 180.0
	worldMinLongitude      = -180.0
	worldMaxLongitude      = 180.0
	worldMinLatitude       = -90.0
	worldMaxLatitude       = 90.0
	webMercatorMaxLatitude = 85.05112878
	tileBase               = 2.0
)

func ReadGeoJSONRequest(body map[string]any) (any, []int, int, string, int, int, error) {
	if body == nil {
		return nil, nil, 0, "", 0, 0, errors.New(
			"body должен быть отображаемым объектом (Mapping), например dict",
		)
	}

	geoJSONRaw, ok := body["geojson"]
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("отсутствует поле 'geojson' в теле запроса")
	}

	zLevelsRaw, ok := body["z_levels"]
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("отсутствует поле 'z_levels' в теле запроса")
	}

	zPatchRaw, ok := body["z_patch"]
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("отсутствует поле 'z_patch' в теле запроса")
	}

	taskUUIDRaw, ok := body["uuid"]
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("отсутствует поле 'uuid' в теле запроса")
	}

	areaIDRaw, ok := body["area_id"]
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("отсутствует поле 'area_id' в теле запроса")
	}

	layerIDRaw, ok := body["layer_id"]
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("отсутствует поле 'layer_id' в теле запроса")
	}

	zLevels, err := parseIntSlice(zLevelsRaw, "z_levels")
	if err != nil {
		return nil, nil, 0, "", 0, 0, err
	}

	zPatch, err := parseInt(zPatchRaw, "z_patch")
	if err != nil {
		return nil, nil, 0, "", 0, 0, err
	}

	taskUUID, ok := taskUUIDRaw.(string)
	if !ok || taskUUID == "" {
		return nil, nil, 0, "", 0, 0, errors.New("uuid должен быть строкой (str)")
	}

	areaID, err := parseInt(areaIDRaw, "area_id")
	if err != nil {
		return nil, nil, 0, "", 0, 0, err
	}

	layerID, err := parseInt(layerIDRaw, "layer_id")
	if err != nil {
		return nil, nil, 0, "", 0, 0, err
	}

	geoJSONMap, ok := geoJSONRaw.(map[string]any)
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("geojson должен быть объектом")
	}

	if geoJSONMap["type"] != "FeatureCollection" {
		return nil, nil, 0, "", 0, 0, errors.New(
			"ожидается GeoJSON типа 'FeatureCollection' в поле 'geojson'",
		)
	}

	rawFeatures, ok := geoJSONMap["features"].([]any)
	if !ok {
		return nil, nil, 0, "", 0, 0, errors.New("поле 'features' должно быть списком")
	}

	if len(rawFeatures) == 0 {
		return nil, nil, 0, "", 0, 0, errors.New(
			"FeatureCollection должен содержать хотя бы один элемент",
		)
	}

	geometries := make([]geom.Geometry, 0, len(rawFeatures))

	for index, rawFeature := range rawFeatures {
		feature, ok := rawFeature.(map[string]any)
		if !ok {
			return nil, nil, 0, "", 0, 0, fmt.Errorf(
				"feature с индексом %d должен быть объектом",
				index,
			)
		}

		rawGeometry, ok := feature["geometry"]
		if !ok || rawGeometry == nil {
			return nil, nil, 0, "", 0, 0, fmt.Errorf(
				"feature с индексом %d не содержит 'geometry'",
				index,
			)
		}

		geometryBytes, err := json.Marshal(rawGeometry)
		if err != nil {
			return nil, nil, 0, "", 0, 0, fmt.Errorf(
				"не удалось сериализовать geometry с индексом %d: %w",
				index,
				err,
			)
		}

		geometry, err := geom.UnmarshalGeoJSON(geometryBytes)
		if err != nil {
			return nil, nil, 0, "", 0, 0, fmt.Errorf(
				"не удалось разобрать geometry с индексом %d: %w",
				index,
				err,
			)
		}

		geometries = append(geometries, geometry)
	}

	unioned, err := geom.UnionMany(geometries)
	if err != nil {
		return nil, nil, 0, "", 0, 0, fmt.Errorf("не удалось объединить геометрии GeoJSON: %w", err)
	}

	return unioned, zLevels, zPatch, taskUUID, areaID, layerID, nil
}

func GetBelongingTiles(geometry any, zoom int) ([]GeoTile, error) {
	sourceGeometry, ok := geometry.(geom.Geometry)
	if !ok {
		return nil, errors.New("geometry must be geom.Geometry")
	}

	if sourceGeometry.IsEmpty() {
		return []GeoTile{}, nil
	}

	leftGeometry, rightGeometry, err := splitByAntimeridian(sourceGeometry)
	if err != nil {
		return nil, err
	}

	result := make([]GeoTile, 0)
	seen := make(map[string]bool)

	if !leftGeometry.IsEmpty() {
		shiftedLeft := shift360(leftGeometry, -1)

		leftTiles, err := tilesForGeometry(shiftedLeft, zoom)
		if err != nil {
			return nil, fmt.Errorf("get tiles for left geometry: %w", err)
		}

		for _, tile := range leftTiles {
			key := tileKey(tile)
			if !seen[key] {
				seen[key] = true

				result = append(result, tile)
			}
		}
	}

	if !rightGeometry.IsEmpty() {
		rightTiles, err := tilesForGeometry(rightGeometry, zoom)
		if err != nil {
			return nil, fmt.Errorf("get tiles for right geometry: %w", err)
		}

		for _, tile := range rightTiles {
			key := tileKey(tile)
			if !seen[key] {
				seen[key] = true

				result = append(result, tile)
			}
		}
	}

	return result, nil
}

func GetPatches(geometry any, tiles []GeoTile, paddingKm float64) (map[string]any, error) {
	sourceGeometry, ok := geometry.(geom.Geometry)
	if !ok {
		return nil, errors.New("geometry must be geom.Geometry")
	}

	result := make(map[string]any)

	for _, tile := range tiles {
		patchName := fmt.Sprintf("%d_%d_%d", tile.X, tile.Y, tile.Z)

		boxGeometry, err := tileToGeometry(tile)
		if err != nil {
			return nil, fmt.Errorf("create tile geometry for patch %s: %w", patchName, err)
		}

		shiftedBoxGeometry := shift360(boxGeometry, 1)

		shiftedBoxExpanded, err := bufferPoly(shiftedBoxGeometry, paddingKm)
		if err != nil {
			return nil, fmt.Errorf("buffer shifted tile geometry for patch %s: %w", patchName, err)
		}

		boxExpanded, err := bufferPoly(boxGeometry, paddingKm)
		if err != nil {
			return nil, fmt.Errorf("buffer tile geometry for patch %s: %w", patchName, err)
		}

		intersectionA, err := geom.Intersection(sourceGeometry, boxExpanded)
		if err != nil {
			return nil, fmt.Errorf(
				"intersect geometry with expanded tile for patch %s: %w",
				patchName,
				err,
			)
		}

		intersectionB, err := geom.Intersection(sourceGeometry, shiftedBoxExpanded)
		if err != nil {
			return nil, fmt.Errorf(
				"intersect geometry with shifted expanded tile for patch %s: %w",
				patchName,
				err,
			)
		}

		patch, err := geom.UnionMany([]geom.Geometry{
			intersectionA,
			intersectionB,
		})
		if err != nil {
			return nil, fmt.Errorf("union patch intersections for patch %s: %w", patchName, err)
		}

		if patch.IsEmpty() {
			continue
		}

		if geom.Intersects(boxGeometry, sourceGeometry) ||
			geom.Intersects(shiftedBoxGeometry, sourceGeometry) {
			result[patchName] = patch
		}
	}

	return result, nil
}

func SerializeTile(tile GeoTile) models.Tile {
	return models.Tile{
		"x": tile.X,
		"y": tile.Y,
		"z": tile.Z,
	}
}

func parseIntSlice(value any, fieldName string) ([]int, error) {
	rawSlice, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s должен быть списком целых чисел", fieldName)
	}

	result := make([]int, 0, len(rawSlice))
	seen := make(map[int]bool)

	for index, item := range rawSlice {
		parsed, err := parseInt(item, fmt.Sprintf("%s[%d]", fieldName, index))
		if err != nil {
			return nil, err
		}

		if seen[parsed] {
			continue
		}

		seen[parsed] = true

		result = append(result, parsed)
	}

	return result, nil
}

func parseInt(value any, fieldName string) (int, error) {
	switch typedValue := value.(type) {
	case int:
		return typedValue, nil

	case int64:
		return int(typedValue), nil

	case float64:
		if math.Trunc(typedValue) != typedValue {
			return 0, fmt.Errorf("%s должен быть целым числом (int)", fieldName)
		}

		return int(typedValue), nil

	default:
		return 0, fmt.Errorf("%s должен быть целым числом (int)", fieldName)
	}
}

func tilesForGeometry(sourceGeometry geom.Geometry, zoom int) ([]GeoTile, error) {
	west, south, east, north, ok := geometryBounds(sourceGeometry)
	if !ok {
		return []GeoTile{}, nil
	}

	minTileX, minTileY := lonLatToTile(west, north, zoom)
	maxTileX, maxTileY := lonLatToTile(east, south, zoom)
	maxIndex := (1 << zoom) - 1
	minTileX = clampInt(minTileX, maxIndex)
	maxTileX = clampInt(maxTileX, maxIndex)
	minTileY = clampInt(minTileY, maxIndex)
	maxTileY = clampInt(maxTileY, maxIndex)
	result := make([]GeoTile, 0)

	for x := minTileX; x <= maxTileX; x++ {
		for y := minTileY; y <= maxTileY; y++ {
			tile := GeoTile{
				X: x,
				Y: y,
				Z: zoom,
			}

			tileGeometry, err := tileToGeometry(tile)
			if err != nil {
				return nil, err
			}

			if geom.Intersects(tileGeometry, sourceGeometry) {
				result = append(result, tile)
			}
		}
	}

	return result, nil
}

func shift360(sourceGeometry geom.Geometry, direction int) geom.Geometry {
	return sourceGeometry.TransformXY(func(xy geom.XY) geom.XY {
		return geom.XY{
			X: xy.X + float64(direction)*fullCircleDegrees,
			Y: xy.Y,
		}
	})
}

func splitByAntimeridian(sourceGeometry geom.Geometry) (geom.Geometry, geom.Geometry, error) {
	if sourceGeometry.IsEmpty() {
		empty, err := emptyGeometry()
		if err != nil {
			return geom.Geometry{}, geom.Geometry{}, err
		}

		return empty, empty, nil
	}

	west, _, east, _, ok := geometryBounds(sourceGeometry)
	if !ok {
		empty, err := emptyGeometry()
		if err != nil {
			return geom.Geometry{}, geom.Geometry{}, err
		}

		return empty, empty, nil
	}

	if west >= -180 && east <= 180 {
		empty, err := emptyGeometry()
		if err != nil {
			return geom.Geometry{}, geom.Geometry{}, err
		}

		return empty, sourceGeometry, nil
	}

	leftBox, err := polygonFromBounds(
		worldMaxLongitude,
		worldMinLatitude,
		fullCircleDegrees,
		worldMaxLatitude,
	)
	if err != nil {
		return geom.Geometry{}, geom.Geometry{}, err
	}

	rightBox, err := polygonFromBounds(
		worldMinLongitude,
		worldMinLatitude,
		worldMaxLongitude,
		worldMinLatitude,
	)
	if err != nil {
		return geom.Geometry{}, geom.Geometry{}, err
	}

	leftPart, err := geom.Intersection(sourceGeometry, leftBox)
	if err != nil {
		return geom.Geometry{}, geom.Geometry{}, fmt.Errorf(
			"intersect left antimeridian box: %w",
			err,
		)
	}

	rightPart, err := geom.Intersection(sourceGeometry, rightBox)
	if err != nil {
		return geom.Geometry{}, geom.Geometry{}, fmt.Errorf(
			"intersect right antimeridian box: %w",
			err,
		)
	}

	return leftPart, rightPart, nil
}

func bufferPoly(sourceGeometry geom.Geometry, distanceKm float64) (geom.Geometry, error) {
	if sourceGeometry.IsEmpty() {
		return sourceGeometry, nil
	}

	west, south, east, north, ok := geometryBounds(sourceGeometry)
	if !ok {
		return emptyGeometry()
	}

	factor := 111.32
	dlat := distanceKm / factor

	dlon := func(lat float64) float64 {
		cosValue := math.Cos(lat * math.Pi / worldMaxLongitude)
		if cosValue == 0 {
			return 0
		}

		return distanceKm / (factor * cosValue)
	}

	southDLon := dlon(south)
	northDLon := dlon(north)

	return geometryFromPolygonCoords([][][]float64{
		{
			{west - southDLon, south - dlat},
			{east + southDLon, south - dlat},
			{east + northDLon, north + dlat},
			{west - northDLon, north + dlat},
			{west - southDLon, south - dlat},
		},
	})
}

func tileToGeometry(tile GeoTile) (geom.Geometry, error) {
	west, south, east, north := tileBounds(tile)

	return polygonFromBounds(west, south, east, north)
}

func tileBounds(tile GeoTile) (float64, float64, float64, float64) {
	west := tileLon(tile.X, tile.Z)
	east := tileLon(tile.X+1, tile.Z)
	north := tileLat(tile.Y, tile.Z)
	south := tileLat(tile.Y+1, tile.Z)

	return west, south, east, north
}

func tileLon(x int, z int) float64 {
	n := math.Pow(tileBase, float64(z))

	return float64(x)/n*fullCircleDegrees - worldMaxLongitude
}

func tileLat(y int, z int) float64 {
	n := math.Pow(tileBase, float64(z))
	rad := math.Atan(math.Sinh(math.Pi * (1.0 - 2.0*float64(y)/n)))

	return rad * worldMaxLongitude / math.Pi
}

func lonLatToTile(lon float64, lat float64, zoom int) (int, int) {
	lat = math.Max(math.Min(lat, webMercatorMaxLatitude), -webMercatorMaxLatitude)
	n := math.Pow(tileBase, float64(zoom))
	x := int(math.Floor((lon + worldMaxLongitude) / fullCircleDegrees * n))
	latRad := lat * math.Pi / worldMaxLongitude
	y := int(
		math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / tileBase * n),
	)

	return x, y
}

func geometryBounds(
	sourceGeometry geom.Geometry,
) (float64, float64, float64, float64, bool) {
	envelope := sourceGeometry.Envelope()

	minXY, maxXY, ok := envelope.MinMaxXYs()
	if !ok {
		return 0, 0, 0, 0, false
	}

	return minXY.X, minXY.Y, maxXY.X, maxXY.Y, true
}

func polygonFromBounds(
	west float64,
	south float64,
	east float64,
	north float64,
) (geom.Geometry, error) {
	return geometryFromPolygonCoords([][][]float64{
		{
			{west, south},
			{east, south},
			{east, north},
			{west, north},
			{west, south},
		},
	})
}

func geometryFromPolygonCoords(coords [][][]float64) (geom.Geometry, error) {
	polygon := struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}{
		Type:        "Polygon",
		Coordinates: coords,
	}

	data, err := json.Marshal(polygon)
	if err != nil {
		return geom.Geometry{}, fmt.Errorf("marshal polygon: %w", err)
	}

	geometry, err := geom.UnmarshalGeoJSON(data)
	if err != nil {
		return geom.Geometry{}, fmt.Errorf("parse polygon geometry: %w", err)
	}

	return geometry, nil
}

func emptyGeometry() (geom.Geometry, error) {
	empty, err := geom.UnmarshalWKT("GEOMETRYCOLLECTION EMPTY")
	if err != nil {
		return geom.Geometry{}, fmt.Errorf("unmarshal empty geometry: %w", err)
	}

	return empty, nil
}

func tileKey(tile GeoTile) string {
	return fmt.Sprintf("%d_%d_%d", tile.X, tile.Y, tile.Z)
}

func clampInt(value int, maxValue int) int {
	if value > maxValue {
		return maxValue
	}

	return value
}
