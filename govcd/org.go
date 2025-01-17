/*
 * Copyright 2019 VMware, Inc.  All rights reserved.  Licensed under the Apache v2 License.
 */

package govcd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"github.com/vmware/go-vcloud-director/v2/util"
)

type Org struct {
	Org    *types.Org
	client *Client
}

func NewOrg(client *Client) *Org {
	return &Org{
		Org:    new(types.Org),
		client: client,
	}
}

// Given an org with a valid HREF, the function refetches the org
// and updates the user's org data. Otherwise if the function fails,
// it returns an error. Users should use refresh whenever they have
// a stale org due to the creation/update/deletion of a resource
// within the org or the org itself.
func (org *Org) Refresh(ctx context.Context) error {
	if *org == (Org{}) {
		return fmt.Errorf("cannot refresh, Object is empty")
	}

	// Empty struct before a new unmarshal, otherwise we end up with duplicate
	// elements in slices.
	unmarshalledOrg := &types.Org{}

	_, err := org.client.ExecuteRequest(ctx, org.Org.HREF, http.MethodGet,
		"", "error refreshing organization: %s", nil, unmarshalledOrg)
	if err != nil {
		return err
	}
	org.Org = unmarshalledOrg

	// The request was successful
	return nil
}

// Given a valid catalog name, FindCatalog returns a Catalog object.
// If no catalog is found, then returns an empty catalog and no error.
// Otherwise it returns an error.
// Deprecated: use org.GetCatalogByName instead
func (org *Org) FindCatalog(ctx context.Context, catalogName string) (Catalog, error) {

	for _, link := range org.Org.Link {
		if link.Rel == "down" && link.Type == "application/vnd.vmware.vcloud.catalog+xml" && link.Name == catalogName {

			cat := NewCatalog(org.client)

			_, err := org.client.ExecuteRequest(ctx, link.HREF, http.MethodGet,
				"", "error retrieving catalog: %s", nil, cat.Catalog)

			return *cat, err
		}
	}

	return Catalog{}, nil
}

// GetVdcByName if user specifies valid vdc name then this returns a vdc object.
// If no vdc is found, then it returns an empty vdc and no error.
// Otherwise it returns an empty vdc and an error.
// Deprecated: use org.GetVDCByName instead
func (org *Org) GetVdcByName(ctx context.Context, vdcname string) (Vdc, error) {
	for _, link := range org.Org.Link {
		if link.Name == vdcname {
			vdc := NewVdc(org.client)

			_, err := org.client.ExecuteRequest(ctx, link.HREF, http.MethodGet,
				"", "error retrieving vdc: %s", nil, vdc.Vdc)

			return *vdc, err
		}
	}
	return Vdc{}, nil
}

// CreateCatalog creates a catalog with specified name and description
func CreateCatalog(ctx context.Context, client *Client, links types.LinkList, Name, Description string) (AdminCatalog, error) {
	adminCatalog, err := CreateCatalogWithStorageProfile(ctx, client, links, Name, Description, nil)
	if err != nil {
		return AdminCatalog{}, nil
	}
	return *adminCatalog, nil
}

// CreateCatalogWithStorageProfile is like CreateCatalog, but allows to specify storage profile
func CreateCatalogWithStorageProfile(ctx context.Context, client *Client, links types.LinkList, Name, Description string, storageProfiles *types.CatalogStorageProfiles) (*AdminCatalog, error) {
	reqCatalog := &types.Catalog{
		Name:        Name,
		Description: Description,
	}
	vcomp := &types.AdminCatalog{
		Xmlns:                  types.XMLNamespaceVCloud,
		Catalog:                *reqCatalog,
		CatalogStorageProfiles: storageProfiles,
	}

	var createOrgLink *types.Link
	for _, link := range links {
		if link.Rel == "add" && link.Type == types.MimeAdminCatalog {
			util.Logger.Printf("[TRACE] Create org - found the proper link for request, HREF: %s, "+
				"name: %s, type: %s, id: %s, rel: %s \n", link.HREF, link.Name, link.Type, link.ID, link.Rel)
			createOrgLink = link
		}
	}

	if createOrgLink == nil {
		return nil, fmt.Errorf("creating catalog failed to find url")
	}

	catalog := NewAdminCatalog(client)
	_, err := client.ExecuteRequest(ctx, createOrgLink.HREF, http.MethodPost,
		"application/vnd.vmware.admin.catalog+xml", "error creating catalog: %s", vcomp, catalog.AdminCatalog)

	return catalog, err
}

// CreateCatalog creates a catalog with given name and description under
// the given organization. Returns an Catalog that contains a creation
// task.
// API Documentation: https://code.vmware.com/apis/220/vcloud#/doc/doc/operations/POST-CreateCatalog.html
func (org *Org) CreateCatalog(ctx context.Context, name, description string) (Catalog, error) {
	catalog, err := org.CreateCatalogWithStorageProfile(ctx, name, description, nil)
	if err != nil {
		return Catalog{}, err
	}
	return *catalog, nil
}

// CreateCatalogWithStorageProfile is like CreateCatalog but additionally allows to specify storage profiles
func (org *Org) CreateCatalogWithStorageProfile(ctx context.Context, name, description string, storageProfiles *types.CatalogStorageProfiles) (*Catalog, error) {
	catalog := NewCatalog(org.client)
	adminCatalog, err := CreateCatalogWithStorageProfile(ctx, org.client, org.Org.Link, name, description, storageProfiles)
	if err != nil {
		return nil, err
	}
	catalog.Catalog = &adminCatalog.AdminCatalog.Catalog
	return catalog, nil
}

func validateVdcConfiguration(vdcDefinition *types.VdcConfiguration) error {
	if vdcDefinition.Name == "" {
		return errors.New("VdcConfiguration missing required field: Name")
	}
	if vdcDefinition.AllocationModel == "" {
		return errors.New("VdcConfiguration missing required field: AllocationModel")
	}
	if vdcDefinition.ComputeCapacity == nil {
		return errors.New("VdcConfiguration missing required field: ComputeCapacity")
	}
	if len(vdcDefinition.ComputeCapacity) != 1 {
		return errors.New("VdcConfiguration invalid field: ComputeCapacity must only have one element")
	}
	if vdcDefinition.ComputeCapacity[0] == nil {
		return errors.New("VdcConfiguration missing required field: ComputeCapacity[0]")
	}
	if vdcDefinition.ComputeCapacity[0].CPU == nil {
		return errors.New("VdcConfiguration missing required field: ComputeCapacity[0].CPU")
	}
	if vdcDefinition.ComputeCapacity[0].CPU.Units == "" {
		return errors.New("VdcConfiguration missing required field: ComputeCapacity[0].CPU.Units")
	}
	if vdcDefinition.ComputeCapacity[0].Memory == nil {
		return errors.New("VdcConfiguration missing required field: ComputeCapacity[0].Memory")
	}
	if vdcDefinition.ComputeCapacity[0].Memory.Units == "" {
		return errors.New("VdcConfiguration missing required field: ComputeCapacity[0].Memory.Units")
	}
	if vdcDefinition.VdcStorageProfile == nil || len(vdcDefinition.VdcStorageProfile) == 0 {
		return errors.New("VdcConfiguration missing required field: VdcStorageProfile")
	}
	if vdcDefinition.VdcStorageProfile[0].Units == "" {
		return errors.New("VdcConfiguration missing required field: VdcStorageProfile.Units")
	}
	if vdcDefinition.ProviderVdcReference == nil {
		return errors.New("VdcConfiguration missing required field: ProviderVdcReference")
	}
	if vdcDefinition.ProviderVdcReference.HREF == "" {
		return errors.New("VdcConfiguration missing required field: ProviderVdcReference.HREF")
	}
	return nil
}

// GetCatalogByHref  finds a Catalog by HREF
// On success, returns a pointer to the Catalog structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetCatalogByHref(ctx context.Context, catalogHref string) (*Catalog, error) {
	cat := NewCatalog(org.client)

	_, err := org.client.ExecuteRequest(ctx, catalogHref, http.MethodGet,
		"", "error retrieving catalog: %s", nil, cat.Catalog)
	if err != nil {
		return nil, err
	}
	// The request was successful
	return cat, nil
}

// GetCatalogByName  finds a Catalog by Name
// On success, returns a pointer to the Catalog structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetCatalogByName(ctx context.Context, catalogName string, refresh bool) (*Catalog, error) {
	if refresh {
		err := org.Refresh(ctx)
		if err != nil {
			return nil, err
		}
	}
	for _, catalog := range org.Org.Link {
		// Get Catalog HREF
		if catalog.Name == catalogName && catalog.Type == types.MimeCatalog {
			return org.GetCatalogByHref(ctx, catalog.HREF)
		}
	}
	return nil, ErrorEntityNotFound
}

// GetCatalogById finds a Catalog by ID
// On success, returns a pointer to the Catalog structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetCatalogById(ctx context.Context, catalogId string, refresh bool) (*Catalog, error) {
	if refresh {
		err := org.Refresh(ctx)
		if err != nil {
			return nil, err
		}
	}
	for _, catalog := range org.Org.Link {
		// Get Catalog HREF
		if equalIds(catalogId, catalog.ID, catalog.HREF) {
			return org.GetCatalogByHref(ctx, catalog.HREF)
		}
	}
	return nil, ErrorEntityNotFound
}

// GetCatalogByNameOrId finds a Catalog by name or ID
// On success, returns a pointer to the Catalog structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetCatalogByNameOrId(ctx context.Context, identifier string, refresh bool) (*Catalog, error) {
	getByName := func(name string, refresh bool) (interface{}, error) { return org.GetCatalogByName(ctx, name, refresh) }
	getById := func(id string, refresh bool) (interface{}, error) { return org.GetCatalogById(ctx, id, refresh) }
	entity, err := getEntityByNameOrId(getByName, getById, identifier, refresh)
	if entity == nil {
		return nil, err
	}
	return entity.(*Catalog), err
}

// GetVDCByHref finds a VDC by HREF
// On success, returns a pointer to the VDC structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetVDCByHref(ctx context.Context, vdcHref string) (*Vdc, error) {
	vdc := NewVdc(org.client)
	_, err := org.client.ExecuteRequest(ctx, vdcHref, http.MethodGet,
		"", "error retrieving VDC: %s", nil, vdc.Vdc)
	if err != nil {
		return nil, err
	}
	// The request was successful
	return vdc, nil
}

// GetVDCByName finds a VDC by Name
// On success, returns a pointer to the VDC structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetVDCByName(ctx context.Context, vdcName string, refresh bool) (*Vdc, error) {
	if refresh {
		err := org.Refresh(ctx)
		if err != nil {
			return nil, err
		}
	}
	for _, link := range org.Org.Link {
		if link.Name == vdcName && link.Type == types.MimeVDC {
			return org.GetVDCByHref(ctx, link.HREF)
		}
	}
	return nil, ErrorEntityNotFound
}

// GetVDCById finds a VDC by ID
// On success, returns a pointer to the VDC structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetVDCById(ctx context.Context, vdcId string, refresh bool) (*Vdc, error) {
	if refresh {
		err := org.Refresh(ctx)
		if err != nil {
			return nil, err
		}
	}
	for _, link := range org.Org.Link {
		if equalIds(vdcId, link.ID, link.HREF) {
			return org.GetVDCByHref(ctx, link.HREF)
		}
	}
	return nil, ErrorEntityNotFound
}

// GetVDCByNameOrId finds a VDC by name or ID
// On success, returns a pointer to the VDC structure and a nil error
// On failure, returns a nil pointer and an error
func (org *Org) GetVDCByNameOrId(ctx context.Context, identifier string, refresh bool) (*Vdc, error) {
	getByName := func(name string, refresh bool) (interface{}, error) { return org.GetVDCByName(ctx, name, refresh) }
	getById := func(id string, refresh bool) (interface{}, error) { return org.GetVDCById(ctx, id, refresh) }
	entity, err := getEntityByNameOrId(getByName, getById, identifier, refresh)
	if entity == nil {
		return nil, err
	}
	return entity.(*Vdc), err
}

// QueryCatalogList returns a list of catalogs for this organization
func (org *Org) QueryCatalogList(ctx context.Context) ([]*types.CatalogRecord, error) {
	util.Logger.Printf("[DEBUG] QueryCatalogList with org name %s", org.Org.Name)
	queryType := org.client.GetQueryType(types.QtCatalog)
	results, err := org.client.cumulativeQuery(ctx, queryType, nil, map[string]string{
		"type":          queryType,
		"filter":        fmt.Sprintf("orgName==%s", url.QueryEscape(org.Org.Name)),
		"filterEncoded": "true",
	})
	if err != nil {
		return nil, err
	}

	var catalogs []*types.CatalogRecord

	if org.client.IsSysAdmin {
		catalogs = results.Results.AdminCatalogRecord
	} else {
		catalogs = results.Results.CatalogRecord
	}
	util.Logger.Printf("[DEBUG] QueryCatalogList returned with : %#v and error: %s", catalogs, err)
	return catalogs, nil
}

// GetTaskList returns Tasks for Organization and error.
func (org *Org) GetTaskList(ctx context.Context) (*types.TasksList, error) {

	for _, link := range org.Org.Link {
		if link.Rel == "down" && link.Type == "application/vnd.vmware.vcloud.tasksList+xml" {

			tasksList := &types.TasksList{}

			_, err := org.client.ExecuteRequest(ctx, link.HREF, http.MethodGet, "",
				"error getting taskList: %s", nil, tasksList)
			if err != nil {
				return nil, err
			}

			return tasksList, nil
		}
	}

	return nil, fmt.Errorf("link not found")
}
