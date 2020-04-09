// Get information about the communication options for the request URI
package main

import "github.com/go-openapi/spec"

func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "LevelDB Raft",
			Description: "Raft backend using LevelDB of Lraft",
			Contact: &spec.ContactInfo{
				ContactInfoProps: spec.ContactInfoProps{
					Name:  "zk",
					Email: "zk@doe.rp",
					URL:   "http://zk.org",
				},
			},
			License: &spec.License{
				LicenseProps: spec.LicenseProps{
					Name: "MIT",
					URL:  "http://mit.org",
				},
			},
			Version: "1.0.0",
		},
	}
	swo.Tags = []spec.Tag{spec.Tag{TagProps: spec.TagProps{
		Name:        "lraft",
		Description: "Managing lraft"}}}
}
