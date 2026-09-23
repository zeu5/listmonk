package models

import (
	"context"
	"database/sql"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Statement is the common execution surface shared by prepared SQL statements
// and backend-specific Go operations. PostgreSQL uses Statement directly;
// SQLite can use transactional implementations for operations that PostgreSQL
// expresses as data-changing CTEs.
type Statement interface {
	Exec(args ...any) (sql.Result, error)
	Get(dest any, args ...any) error
	Select(dest any, args ...any) error
}

// Queries contains all prepared SQL queries.
type Queries struct {
	GetDashboardCharts Statement `query:"get-dashboard-charts"`
	GetDashboardCounts Statement `query:"get-dashboard-counts"`

	InsertSubscriber                Statement `query:"insert-subscriber"`
	UpsertSubscriber                Statement `query:"upsert-subscriber"`
	UpsertBlocklistSubscriber       Statement `query:"upsert-blocklist-subscriber"`
	GetSubscriber                   Statement `query:"get-subscriber"`
	HasSubscriberLists              Statement `query:"has-subscriber-list"`
	GetSubscribersByEmails          Statement `query:"get-subscribers-by-emails"`
	GetSubscriberLists              Statement `query:"get-subscriber-lists"`
	GetSubscriptions                Statement `query:"get-subscriptions"`
	GetSubscriberListsLazy          Statement `query:"get-subscriber-lists-lazy"`
	UpdateSubscriber                Statement `query:"update-subscriber"`
	UpdateSubscriberWithLists       Statement `query:"update-subscriber-with-lists"`
	BlocklistSubscribers            Statement `query:"blocklist-subscribers"`
	AddSubscribersToLists           Statement `query:"add-subscribers-to-lists"`
	DeleteSubscriptions             Statement `query:"delete-subscriptions"`
	DeleteUnconfirmedSubscriptions  Statement `query:"delete-unconfirmed-subscriptions"`
	ConfirmSubscriptionOptin        Statement `query:"confirm-subscription-optin"`
	UnsubscribeSubscribersFromLists Statement `query:"unsubscribe-subscribers-from-lists"`
	DeleteSubscribers               Statement `query:"delete-subscribers"`
	DeleteBlocklistedSubscribers    Statement `query:"delete-blocklisted-subscribers"`
	DeleteOrphanSubscribers         Statement `query:"delete-orphan-subscribers"`
	UnsubscribeByCampaign           Statement `query:"unsubscribe-by-campaign"`
	ExportSubscriberData            Statement `query:"export-subscriber-data"`
	GetSubscriberActivity           Statement `query:"get-subscriber-activity"`

	// Non-prepared arbitrary subscriber queries.
	QuerySubscribers                       string    `query:"query-subscribers"`
	QuerySubscribersCount                  string    `query:"query-subscribers-count"`
	QuerySubscribersCountAll               Statement `query:"query-subscribers-count-all"`
	QuerySubscribersForExport              string    `query:"query-subscribers-for-export"`
	QuerySubscribersTpl                    string    `query:"query-subscribers-template"`
	DeleteSubscribersByQuery               string    `query:"delete-subscribers-by-query"`
	AddSubscribersToListsByQuery           string    `query:"add-subscribers-to-lists-by-query"`
	BlocklistSubscribersByQuery            string    `query:"blocklist-subscribers-by-query"`
	DeleteSubscriptionsByQuery             string    `query:"delete-subscriptions-by-query"`
	UnsubscribeSubscribersFromListsByQuery string    `query:"unsubscribe-subscribers-from-lists-by-query"`

	CreateList      Statement `query:"create-list"`
	QueryLists      string    `query:"query-lists"`
	GetLists        Statement `query:"get-lists"`
	GetListsByOptin Statement `query:"get-lists-by-optin"`
	GetListTypes    Statement `query:"get-list-types"`
	UpdateList      Statement `query:"update-list"`
	UpdateListsDate Statement `query:"update-lists-date"`
	DeleteLists     Statement `query:"delete-lists"`

	CreateCampaign        Statement `query:"create-campaign"`
	QueryCampaigns        string    `query:"query-campaigns"`
	GetCampaign           Statement `query:"get-campaign"`
	GetCampaignForPreview Statement `query:"get-campaign-for-preview"`
	GetCampaignStats      Statement `query:"get-campaign-stats"`
	GetCampaignStatus     Statement `query:"get-campaign-status"`
	GetArchivedCampaigns  Statement `query:"get-archived-campaigns"`
	CampaignHasLists      Statement `query:"campaign-has-lists"`

	// These two queries are read as strings and based on settings.individual_tracking=on/off,
	// are interpolated and copied to view and click counts. Same query, different tables.
	GetCampaignAnalyticsCounts string    `query:"get-campaign-analytics-counts"`
	GetCampaignViewCounts      Statement `query:"get-campaign-view-counts"`
	GetCampaignClickCounts     Statement `query:"get-campaign-click-counts"`
	GetCampaignLinkCounts      Statement `query:"get-campaign-link-counts"`
	GetCampaignBounceCounts    Statement `query:"get-campaign-bounce-counts"`
	DeleteCampaignViews        Statement `query:"delete-campaign-views"`
	DeleteCampaignLinkClicks   Statement `query:"delete-campaign-link-clicks"`
	ExportCampaignViews        Statement `query:"export-campaign-views"`
	ExportCampaignLinkClicks   Statement `query:"export-campaign-link-clicks"`

	NextCampaigns            Statement `query:"next-campaigns"`
	GetRunningCampaign       Statement `query:"get-running-campaign"`
	NextCampaignSubscribers  Statement `query:"next-campaign-subscribers"`
	GetOneCampaignSubscriber Statement `query:"get-one-campaign-subscriber"`
	UpdateCampaign           Statement `query:"update-campaign"`
	UpdateCampaignStatus     Statement `query:"update-campaign-status"`
	UpdateCampaignCounts     Statement `query:"update-campaign-counts"`
	UpdateCampaignArchive    Statement `query:"update-campaign-archive"`
	RegisterCampaignView     Statement `query:"register-campaign-view"`
	DeleteCampaign           Statement `query:"delete-campaign"`
	DeleteCampaigns          Statement `query:"delete-campaigns"`

	InsertMedia Statement `query:"insert-media"`
	GetMedia    Statement `query:"get-media"`
	QueryMedia  Statement `query:"query-media"`
	DeleteMedia Statement `query:"delete-media"`

	CreateTemplate     Statement `query:"create-template"`
	GetTemplates       Statement `query:"get-templates"`
	UpdateTemplate     Statement `query:"update-template"`
	SetDefaultTemplate Statement `query:"set-default-template"`
	DeleteTemplate     Statement `query:"delete-template"`

	CreateLink        Statement `query:"create-link"`
	GetLinkURL        Statement `query:"get-link-url"`
	RegisterLinkClick Statement `query:"register-link-click"`

	GetSettings         Statement `query:"get-settings"`
	UpdateSettings      Statement `query:"update-settings"`
	UpdateSettingsByKey Statement `query:"update-settings-by-key"`

	// GetStats Statement `query:"get-stats"`
	RecordBounce                Statement `query:"record-bounce"`
	QueryBounces                string    `query:"query-bounces"`
	BlocklistBouncedSubscribers Statement `query:"blocklist-bounced-subscribers"`
	DeleteBounces               Statement `query:"delete-bounces"`
	DeleteBouncesBySubscriber   Statement `query:"delete-bounces-by-subscriber"`
	GetDBInfo                   string    `query:"get-db-info"`

	CreateUser         Statement `query:"create-user"`
	UpdateUser         Statement `query:"update-user"`
	UpdateUserProfile  Statement `query:"update-user-profile"`
	UpdateUserLogin    Statement `query:"update-user-login"`
	SetUserTwoFA       Statement `query:"set-user-twofa"`
	DeleteUsers        Statement `query:"delete-users"`
	GetUsers           Statement `query:"get-users"`
	GetUser            Statement `query:"get-user"`
	GetAPITokens       Statement `query:"get-api-tokens"`
	LoginUser          Statement `query:"login-user"`
	DeleteUserSessions Statement `query:"delete-user-sessions"`

	CreateRole            Statement `query:"create-role"`
	GetUserRoles          Statement `query:"get-user-roles"`
	GetListRoles          Statement `query:"get-list-roles"`
	UpdateRole            Statement `query:"update-role"`
	DeleteRole            Statement `query:"delete-role"`
	UpsertListPermissions Statement `query:"upsert-list-permissions"`
	DeleteListPermission  Statement `query:"delete-list-permission"`
}

// compileSubscriberQueryTpl takes an arbitrary WHERE expressions
// to filter subscribers from the subscribers table and prepares a query
// out of it using the raw `query-subscribers-template` query template.
// While doing this, a readonly transaction is created and the query is
// dry run on it to ensure that it is indeed readonly.
func (q *Queries) compileSubscriberQueryTpl(searchStr, queryExp string, db *sqlx.DB, subStatus string) (string, error) {
	tx, err := db.BeginTxx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// There's an arbitrary query condition.
	cond := "TRUE"
	if queryExp != "" {
		cond = queryExp
	}

	// Perform the dry run.
	stmt := strings.ReplaceAll(q.QuerySubscribersTpl, "%query%", cond)
	if _, err := tx.Exec(stmt, true, pq.Int64Array{}, subStatus, searchStr); err != nil {
		return "", err
	}

	return stmt, nil
}

// compileSubscriberQueryTpl takes an arbitrary WHERE expressions and a subscriber
// query template that depends on the filter (eg: delete by query, blocklist by query etc.)
// combines and executes them.
func (q *Queries) ExecSubQueryTpl(searchStr, queryExp, baseQueryTpl string, listIDs []int, db *sqlx.DB, subStatus string, args ...any) error {
	// Perform a dry run.
	filterExp, err := q.compileSubscriberQueryTpl(searchStr, queryExp, db, subStatus)
	if err != nil {
		return err
	}

	if len(listIDs) == 0 {
		listIDs = []int{}
	}

	// Insert the subscriber filter query into the target query.
	stmt := strings.ReplaceAll(baseQueryTpl, "%query%", filterExp)

	// First argument is the boolean indicating if the query is a dry run.
	a := append([]any{false, pq.Array(listIDs), subStatus, searchStr}, args...)

	// Execute the query on the DB.
	if _, err := db.Exec(stmt, a...); err != nil {
		return err
	}
	return nil
}
