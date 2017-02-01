package tools

import "time"

type JSONHeader struct {
	UUID string `json:"uuid"`
	//ID           json.ObjectId `json:"_id,omitempty"`
	//HeaderUUID string `json:"uuid"`
	//Owner        string    `json:"owner,omitempty"`
	Revision     int        `json:"revision"`
	Header       string     `json:"header"`
	Handle       string     `json:"handle"`
	Active       bool       `json:"active"`
	CreationDate *time.Time `json:"creation_date"`
	//UpdateDate   time.Time `json:"update_date"`
	Data *map[string]interface{} `json:"data,omitempty"`
}

type JSONEntry struct {
	UUID string `json:"uuid"`
	//ID         json.ObjectId `json:"_id,omitempty"`
	//EntryUUID  string `json:"uuid"`
	Revision   int                     `json:"revision"`
	HeaderUUID string                  `json:"header_uuid"`
	Start      *time.Time              `json:"start"`
	End        *time.Time              `json:"end,omitempty"`
	Data       *map[string]interface{} `json:"data,omitempty"`
	//UpdateDate time.Time  `json:"update_date"`
}
