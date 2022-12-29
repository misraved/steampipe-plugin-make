package make

import (
	"context"
	"fmt"
	"github.com/marekjalovec/steampipe-plugin-make/make/client"
	"github.com/marekjalovec/steampipe-plugin-make/make/utils"
	"github.com/turbot/steampipe-plugin-sdk/v4/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v4/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v4/plugin/transform"
	"strconv"
)

type Team struct {
	Id             int    `json:"id"`
	Name           string `json:"name"`
	OrganizationId int    `json:"organizationId"`
}

type TeamResponse struct {
	Team Team `json:"team"`
}

type TeamListResponse struct {
	Teams      []Team     `json:"teams"`
	Pagination Pagination `json:"pg"`
}

func tableTeam(_ context.Context) *plugin.Table {
	return &plugin.Table{
		Name:        "make_team",
		Description: "Teams are containers that contain scenarios and data accessible only by the members of the team.",
		List: &plugin.ListConfig{
			Hydrate:       listTeams,
			ParentHydrate: listOrganizations,
		},
		Get: &plugin.GetConfig{
			KeyColumns: plugin.SingleColumn("id"),
			Hydrate:    getTeam,
		},
		Columns: []*plugin.Column{
			// Key Columns
			{Name: "id", Type: proto.ColumnType_INT, Description: "The team ID."},

			// Other Columns
			{Name: "name", Type: proto.ColumnType_STRING, Description: "The name of the team."},
			{Name: "organization_id", Type: proto.ColumnType_INT, Description: "The ID of the organization."},

			// Standard Columns
			{Name: "title", Type: proto.ColumnType_STRING, Description: utils.StandardColumnDescription("title"), Transform: transform.FromField("Name")}},
	}
}

func getTeam(_ context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	utils.LogQueryContext("getTeam", d, h)

	var logger = utils.GetLogger()

	c, err := client.GetClient(d.Connection)
	if err != nil {
		return nil, err
	}

	var id = d.KeyColumnQuals["id"].GetInt64Value()
	config := client.NewRequestConfig(EndpointTeam, id)
	utils.ColumnsToParams(&config.Params, ColumnsTeam())

	var result = &TeamResponse{}
	err = c.Get(&config, &result)
	if err != nil {
		logger.Info("getTeam", err.Error())
		return nil, err
	}

	return result.Team, nil
}

func listTeams(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	utils.LogQueryContext("listTeams", d, h)

	if h.Item == nil {
		return nil, fmt.Errorf("no parent item found")
	}

	var logger = utils.GetLogger()

	// create new Make client
	c, err := client.GetClient(d.Connection)
	if err != nil {
		return nil, err
	}

	var config = client.NewRequestConfig(EndpointTeam, 0)
	utils.ColumnsToParams(&config.Params, ColumnsTeam())
	config.Params.Set("organizationId", strconv.Itoa(h.Item.(Organization).Id))
	if d.QueryContext.Limit != nil {
		config.Pagination.Limit = *d.QueryContext.Limit
	}

	var pagesLeft = true
	for pagesLeft {
		var result = &TeamListResponse{}
		err = c.Get(&config, result)
		if err != nil {
			logger.Error("make_team.listTeams", "connection_error", err)
			return nil, err
		}

		// stream results
		for _, i := range result.Teams {
			d.StreamListItem(ctx, i)
		}

		// pagination
		var resultCount = int64(len(result.Teams))
		if d.QueryStatus.RowsRemaining(ctx) <= 0 || resultCount < config.Pagination.Limit {
			pagesLeft = false
		} else {
			config.Pagination.Offset += config.Pagination.Limit
		}
	}

	return nil, nil
}