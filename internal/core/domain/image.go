package domain

import "time"

// Image is the engine-agnostic representation of a local Docker image.
type Image struct {
	ID          string
	RepoTags    []string
	RepoDigests []string
	Size        int64
	Containers  int64 // number of containers using this image, -1 if unknown
	Created     time.Time
	Labels      map[string]string
}

// ImageLayer is one row of `docker history`.
type ImageLayer struct {
	ID        string
	Created   time.Time
	CreatedBy string
	Size      int64
	Tags      []string
	Comment   string
}
