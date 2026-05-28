package models

type Tile = map[string]any

type PatchMessage struct {
	Name        string            `json:"name"`
	TilesByZoom map[string][]Tile `json:"tilesByZoom"`
	AreaID      int               `json:"areaId"`
	LayerID     int               `json:"layerId"`
	TaskUUID    string            `json:"taskUuid"`
	PatchUUID   string            `json:"patchUuid"`
}

type ProgressMessage struct {
	EventID        string `json:"eventId"`
	TaskID         string `json:"taskId"`
	PatchID        string `json:"patchId"`
	Status         string `json:"status"`
	CompletedTiles int    `json:"completedTiles"`
	TotalTiles     int    `json:"totalTiles"`
	ErrorTiles     int    `json:"errorTiles"`
	Timestamp      string `json:"ts"`
}

type DLQMessage struct {
	Advisory map[string]any `json:"advisory"`
	Original any            `json:"original"`
	Consumer any            `json:"consumer"`
}

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string            `json:"type"`
	Geometry   any               `json:"geometry"`
	Properties FeatureProperties `json:"properties"`
}

type FeatureProperties struct {
	PatchName  string `json:"patchName"`
	PatchUUID  string `json:"patchUuid"`
	TotalTiles int    `json:"totalTiles"`
}
