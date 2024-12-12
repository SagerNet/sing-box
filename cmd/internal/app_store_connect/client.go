package main

import (
	"context"
	"fmt"
	_ "unsafe"

	"github.com/cidertool/asc-go/asc"
)

type Client struct {
	*asc.Client
}

func (c *Client) UpdateBuildForAppStoreVersion(ctx context.Context, id string, buildID *string) (*asc.Response, error) {
	linkage := newRelationshipDeclaration(buildID, "builds")
	url := fmt.Sprintf("appStoreVersions/%s/relationships/build", id)
	return c.patch(ctx, url, newRequestBody(linkage), nil)
}

func newRelationshipDeclaration(id *string, relationshipType string) *asc.RelationshipData {
	if id == nil {
		return nil
	}

	return &asc.RelationshipData{
		ID:   *id,
		Type: relationshipType,
	}
}
