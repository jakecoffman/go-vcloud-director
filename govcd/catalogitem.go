/*
 * Copyright 2019 VMware, Inc.  All rights reserved.  Licensed under the Apache v2 License.
 */

package govcd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"github.com/vmware/go-vcloud-director/v2/util"
)

type CatalogItem struct {
	CatalogItem *types.CatalogItem
	client      *Client
}

func NewCatalogItem(cli *Client) *CatalogItem {
	return &CatalogItem{
		CatalogItem: new(types.CatalogItem),
		client:      cli,
	}
}

func (catalogItem *CatalogItem) GetVAppTemplate(ctx context.Context) (VAppTemplate, error) {

	cat := NewVAppTemplate(catalogItem.client)

	_, err := catalogItem.client.ExecuteRequest(ctx, catalogItem.CatalogItem.Entity.HREF, http.MethodGet,
		"", "error retrieving vApp template: %s", nil, cat.VAppTemplate)

	// The request was successful
	return *cat, err

}

// Deletes the Catalog Item, returning an error if the vCD call fails.
// Link to API call: https://code.vmware.com/apis/220/vcloud#/doc/doc/operations/DELETE-CatalogItem.html
func (catalogItem *CatalogItem) Delete(ctx context.Context) error {
	util.Logger.Printf("[TRACE] Deleting catalog item: %#v", catalogItem.CatalogItem)
	catalogItemHREF := catalogItem.client.VCDHREF
	catalogItemHREF.Path += "/catalogItem/" + catalogItem.CatalogItem.ID[23:]

	util.Logger.Printf("[TRACE] Url for deleting catalog item: %#v and name: %s", catalogItemHREF, catalogItem.CatalogItem.Name)

	return catalogItem.client.ExecuteRequestWithoutResponse(ctx, catalogItemHREF.String(), http.MethodDelete,
		"", "error deleting Catalog item: %s", nil)
}

// queryCatalogItemList returns a list of Catalog Item for the given parent
func queryCatalogItemList(ctx context.Context, client *Client, parentField, parentValue string) ([]*types.QueryResultCatalogItemType, error) {

	catalogItemType := types.QtCatalogItem
	if client.IsSysAdmin {
		catalogItemType = types.QtAdminCatalogItem
	}

	filterText := fmt.Sprintf("%s==%s", parentField, url.QueryEscape(parentValue))

	results, err := client.cumulativeQuery(ctx, catalogItemType, nil, map[string]string{
		"type":   catalogItemType,
		"filter": filterText,
	})
	if err != nil {
		return nil, fmt.Errorf("error querying catalog items %s", err)
	}

	if client.IsSysAdmin {
		return results.Results.AdminCatalogItemRecord, nil
	} else {
		return results.Results.CatalogItemRecord, nil
	}
}

// QueryCatalogItemList returns a list of Catalog Item for the given catalog
func (catalog *Catalog) QueryCatalogItemList(ctx context.Context) ([]*types.QueryResultCatalogItemType, error) {
	return queryCatalogItemList(ctx, catalog.client, "catalog", catalog.Catalog.ID)
}

// QueryCatalogItemList returns a list of Catalog Item for the given VDC
func (vdc *Vdc) QueryCatalogItemList(ctx context.Context) ([]*types.QueryResultCatalogItemType, error) {
	return queryCatalogItemList(ctx, vdc.client, "vdc", vdc.Vdc.ID)
}

// QueryCatalogItemList returns a list of Catalog Item for the given Admin VDC
func (vdc *AdminVdc) QueryCatalogItemList(ctx context.Context) ([]*types.QueryResultCatalogItemType, error) {
	return queryCatalogItemList(ctx, vdc.client, "vdc", vdc.AdminVdc.ID)
}

// queryVappTemplateList returns a list of vApp templates for the given parent
func queryVappTemplateList(ctx context.Context, client *Client, parentField, parentValue string) ([]*types.QueryResultVappTemplateType, error) {

	vappTemplateType := types.QtVappTemplate
	if client.IsSysAdmin {
		vappTemplateType = types.QtAdminVappTemplate
	}
	results, err := client.cumulativeQuery(ctx, vappTemplateType, nil, map[string]string{
		"type":   vappTemplateType,
		"filter": fmt.Sprintf("%s==%s", parentField, url.QueryEscape(parentValue)),
	})
	if err != nil {
		return nil, fmt.Errorf("error querying vApp templates %s", err)
	}

	if client.IsSysAdmin {
		return results.Results.AdminVappTemplateRecord, nil
	} else {
		return results.Results.VappTemplateRecord, nil
	}
}

// QueryVappTemplateList returns a list of vApp templates for the given VDC
func (vdc *Vdc) QueryVappTemplateList(ctx context.Context) ([]*types.QueryResultVappTemplateType, error) {
	return queryVappTemplateList(ctx, vdc.client, "vdcName", vdc.Vdc.Name)
}

// QueryVappTemplateList returns a list of vApp templates for the given VDC
func (vdc *AdminVdc) QueryVappTemplateList(ctx context.Context) ([]*types.QueryResultVappTemplateType, error) {
	return queryVappTemplateList(ctx, vdc.client, "vdcName", vdc.AdminVdc.Name)
}

// QueryVappTemplateList returns a list of vApp templates for the given catalog
func (catalog *Catalog) QueryVappTemplateList(ctx context.Context) ([]*types.QueryResultVappTemplateType, error) {
	return queryVappTemplateList(ctx, catalog.client, "catalogName", catalog.Catalog.Name)
}
