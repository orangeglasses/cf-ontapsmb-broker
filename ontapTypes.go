package main

import (
	"time"

	"github.com/pivotal-cf/brokerapi/v7/domain"
)

type Aggregate struct {
	Name string `json:"name,omitempty"`
	UUID string `json:"uuid,omitempty"`
}

type Volume struct {
	Aggregates []Aggregate `json:"aggregates"`
	Comment    string      `json:"comment"`
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Svm        struct {
		UUID string `json:"uuid,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"svm"`
	Nas struct {
		GID          int    `json:"gid"`
		UID          int    `json:"uid"`
		Path         string `json:"path"`
		ExportPolicy struct {
			Name string `json:"name"`
		} `json:"export_policy"`
	} `json:"nas"`
}

type CifsAccess struct {
	Access      string `json:"access"`
	UserOrGroup string `json:"user_or_group"`
}

type ApplicationComponents struct {
	Name       string `json:"name"`
	TotalSize  int64  `json:"total_size"`
	ShareCount int    `json:"share_count"`
	ScaleOut   bool   `json:"scale_out"`
	Tiering    struct {
		Control string `json:"control"`
	} `json:"tiering"`
	StorageService struct {
		Name string `json:"name"`
	} `json:"storage_service"`
}

type CifsApplication struct {
	Name           string `json:"name"`
	SmartContainer bool   `json:"smart_container"`
	Svm            struct {
		Name string `json:"name"`
	} `json:"svm"`
	Nas struct {
		NfsAccess             []interface{}           `json:"nfs_access"`
		CifsAccess            []CifsAccess            `json:"cifs_access"`
		ApplicationComponents []ApplicationComponents `json:"application_components"`
		ProtectionType        struct {
			RemoteRpo   string `json:"remote_rpo"`
			LocalPolicy string `json:"local_policy"`
		} `json:"protection_type"`
	} `json:"nas"`
	Template struct {
		Name string `json:"name"`
	} `json:"template"`
}

type AcceptResponse struct {
	Job struct {
		UUID  string `json:"uuid"`
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"_links"`
	} `json:"job"`
}

type JobStatus struct {
	UUID        string    `json:"uuid"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	Message     string    `json:"message"`
	Code        int       `json:"code"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Links       struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"_links"`
}

// maps ontap status to broker status
var statusMap = map[string]domain.LastOperationState{
	"success": domain.Succeeded,
	"running": domain.InProgress,
	"failure": domain.Failed,
}

type ResultList struct {
	Records []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"records"`
	NumRecords int `json:"num_records"`
}

type cifsACL struct {
	UserOrGroup string `json:"user_or_group"`
	Type        string `json:"type"`
	Permission  string `json:"permission"`
}
