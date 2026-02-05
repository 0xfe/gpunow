package gcp

import (
	"fmt"
	"path"
	"strings"
)

func ZoneResource(project, zone, resourceType, name string) string {
	return fmt.Sprintf("projects/%s/zones/%s/%s/%s", project, zone, resourceType, name)
}

func GlobalResource(project, resourceType, name string) string {
	return fmt.Sprintf("projects/%s/global/%s/%s", project, resourceType, name)
}

func RegionResource(project, region, resourceType, name string) string {
	return fmt.Sprintf("projects/%s/regions/%s/%s/%s", project, region, resourceType, name)
}

func ShortName(resource string) string {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return ""
	}
	resource = strings.TrimRight(resource, "/")
	return path.Base(resource)
}
