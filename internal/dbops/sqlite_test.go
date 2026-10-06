package dbops

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/knadh/goyesql/v2"
	"github.com/knadh/listmonk/internal/dbconn"
	"github.com/knadh/listmonk/internal/dbtypes"
	"github.com/knadh/listmonk/models"
	"github.com/lib/pq"
	null "gopkg.in/volatiletech/null.v6"
)

func TestSQLiteSubscriberAndCampaignOperations(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/ops.db"})
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	schema, err := os.ReadFile("../../schema-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	var defaultTemplateID int
	if err = db.Get(&defaultTemplateID, `INSERT INTO templates(name,type,subject,body,body_source,is_default)
		VALUES('Default','campaign','Subject','Default template body',NULL,1) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	var listID int
	if err = db.Get(&listID, "INSERT INTO lists(uuid,name,type,optin,status,tags) VALUES('list-uuid','Test','public','single','active','{}') RETURNING id"); err != nil {
		t.Fatal(err)
	}
	ops := SQLite(db)
	var sub struct {
		UUID string `db:"uuid"`
		ID   int    `db:"id"`
	}
	if err = ops["upsert-subscriber"].Get(&sub, "sub-uuid", "person@example.com", "Person", []byte(`{"ok":true}`), pq.Array([]int{listID}), models.SubscriptionStatusConfirmed, true, true); err != nil {
		t.Fatal(err)
	}
	var subscriptions int
	if err = db.Get(&subscriptions, "SELECT count(*) FROM subscriber_lists WHERE subscriber_id=? AND list_id=?", sub.ID, listID); err != nil || subscriptions != 1 {
		t.Fatalf("subscriptions=%d err=%v", subscriptions, err)
	}
	var campaignID int
	args := []any{uuid.Must(uuid.NewV4()), "regular", "Campaign", "Subject", "sender@example.com", "", null.String{}, "richtext", null.Time{}, models.Headers{}, models.JSON{}, pq.StringArray{"tag"}, "email", null.Int{}, pq.Array([]int{listID}), false, null.String{}, null.Int{}, dbtypes.RawJSON(`{}`), pq.Array([]int{}), null.String{}}
	if err = ops["create-campaign"].Get(&campaignID, args...); err != nil {
		t.Fatal(err)
	}
	var created models.Campaign
	if err = db.Get(&created, `SELECT campaigns.*,
		(SELECT body FROM templates WHERE id=campaigns.template_id) AS template_body
		FROM campaigns WHERE id=?`, campaignID); err != nil {
		t.Fatal(err)
	}
	if !created.TemplateID.Valid || created.TemplateID.Int != defaultTemplateID || created.TemplateBody != "Default template body" {
		t.Fatalf("default template not selected: template_id=%#v template_body=%q", created.TemplateID, created.TemplateBody)
	}
	var visualTemplateID int
	if err = db.Get(&visualTemplateID, `INSERT INTO templates(name,type,subject,body,body_source)
		VALUES('Visual','campaign_visual','Subject','Visual body','{"blocks":[]}') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	visualArgs := append([]any(nil), args...)
	visualArgs[0] = uuid.Must(uuid.NewV4())
	visualArgs[2] = "Visual campaign"
	visualArgs[7] = "visual"
	visualArgs[13] = null.NewInt(visualTemplateID, true)
	var visualCampaignID int
	if err = ops["create-campaign"].Get(&visualCampaignID, visualArgs...); err != nil {
		t.Fatal(err)
	}
	var visual struct {
		Body       string      `db:"body"`
		BodySource null.String `db:"body_source"`
		TemplateID null.Int    `db:"template_id"`
	}
	if err = db.Get(&visual, "SELECT body,body_source,template_id FROM campaigns WHERE id=?", visualCampaignID); err != nil {
		t.Fatal(err)
	}
	if visual.Body != "Visual body" || !visual.BodySource.Valid || visual.BodySource.String != `{"blocks":[]}` || visual.TemplateID.Valid {
		t.Fatalf("visual template not imported: %#v", visual)
	}
	var links int
	if err = db.Get(&links, "SELECT count(*) FROM campaign_lists WHERE campaign_id=? AND list_id=?", campaignID, listID); err != nil || links != 1 {
		t.Fatalf("campaign lists=%d err=%v", links, err)
	}
	if _, err = ops["update-campaign"].Exec(campaignID, "Campaign updated", "Subject updated", "sender@example.com", "Updated body", null.NewString("Alt", true), "richtext", null.Time{}, models.Headers{{"X-Test": "ok"}}, models.JSON{"utm": "campaign"}, pq.StringArray{"tag", "updated"}, "email", null.Int{}, pq.Array([]int{listID}), true, null.NewString("campaign-updated", true), null.Int{}, dbtypes.RawJSON(`{"title":"Campaign"}`), pq.Array([]int{}), null.String{}); err != nil {
		t.Fatal(err)
	}
	var updated models.Campaign
	if err = db.Get(&updated, "SELECT campaigns.*, '' AS template_body FROM campaigns WHERE id=?", campaignID); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Campaign updated" || updated.Body != "Updated body" || !updated.Archive {
		t.Fatalf("campaign not updated: %#v", updated)
	}
	var subscribers []models.Subscriber
	if err = ops["next-campaign-subscribers"].Select(&subscribers, campaignID, "regular", 0, sub.ID, pq.Array([]int{listID}), 100); err != nil {
		t.Fatal(err)
	}
	if len(subscribers) != 1 || subscribers[0].ID != sub.ID {
		t.Fatalf("subscribers=%#v", subscribers)
	}
	queriesFile, err := os.ReadFile("../../queries-sqlite/campaigns.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(queriesFile)
	if err != nil {
		t.Fatal(err)
	}
	var fetched models.Campaign
	if err = db.Get(&fetched, queries["get-campaign"].Query, campaignID, nil, "", "default"); err != nil {
		t.Fatal(err)
	}
	if len(fetched.Lists) == 0 || len(fetched.Media) == 0 {
		t.Fatalf("campaign relationships missing: lists=%s media=%s", fetched.Lists, fetched.Media)
	}
	if _, err = db.Exec("UPDATE campaigns SET status='running' WHERE id=?", campaignID); err != nil {
		t.Fatal(err)
	}
	var campaigns []*models.Campaign
	if err = ops["next-campaigns"].Select(&campaigns, pq.Array([]int64{}), pq.Array([]int64{})); err != nil {
		t.Fatal(err)
	}
	if len(campaigns) != 1 || campaigns[0].ID != campaignID {
		t.Fatalf("campaigns=%#v", campaigns)
	}

	listsData, err := os.ReadFile("../../queries-sqlite/lists.sql")
	if err != nil {
		t.Fatal(err)
	}
	listsQueries, err := goyesql.ParseBytes(listsData)
	if err != nil {
		t.Fatal(err)
	}
	listQuery := strings.ReplaceAll(listsQueries["query-lists"].Query, "%order%", "ls.id")
	var lists []models.List
	if err = db.Select(&lists, listQuery, 0, "", "", "", "", "", pq.Array([]string{}), true, pq.Array([]int{}), 0, 10); err != nil {
		t.Fatal(err)
	}
	if len(lists) != 1 || lists[0].ID != listID || lists[0].SubscriberCount != 1 {
		t.Fatalf("lists=%#v", lists)
	}

	subscriberData, err := os.ReadFile("../../queries-sqlite/subscribers.sql")
	if err != nil {
		t.Fatal(err)
	}
	subscriberQueries, err := goyesql.ParseBytes(subscriberData)
	if err != nil {
		t.Fatal(err)
	}
	var export models.SubscriberExportProfile
	if err = db.Get(&export, subscriberQueries["export-subscriber-data"].Query, sub.ID, ""); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]dbtypes.RawJSON{
		"profile": export.Profile, "subscriptions": export.Subscriptions,
		"campaign_views": export.CampaignViews, "link_clicks": export.LinkClicks,
	} {
		if !json.Valid(value) {
			t.Fatalf("invalid %s JSON: %q", name, value)
		}
	}
	var activity models.SubscriberActivity
	if err = db.Get(&activity, subscriberQueries["get-subscriber-activity"].Query, sub.ID); err != nil {
		t.Fatal(err)
	}
	if string(activity.CampaignViews) != "[]" || string(activity.LinkClicks) != "[]" {
		t.Fatalf("empty activity = %#v", activity)
	}

	if _, err = db.Exec("INSERT INTO bounces(subscriber_id,campaign_id,type,source,meta) VALUES(?,?,?,?,?)",
		sub.ID, campaignID, "hard", "test", dbtypes.RawJSON(`{"message":"bounced"}`)); err != nil {
		t.Fatal(err)
	}
	bounceData, err := os.ReadFile("../../queries-sqlite/bounces.sql")
	if err != nil {
		t.Fatal(err)
	}
	bounceQueries, err := goyesql.ParseBytes(bounceData)
	if err != nil {
		t.Fatal(err)
	}
	bounceQuery := strings.ReplaceAll(bounceQueries["query-bounces"].Query, "%order%", "b.id")
	var bounces []models.Bounce
	if err = db.Select(&bounces, bounceQuery, 0, campaignID, 0, "", 0, 10); err != nil {
		t.Fatal(err)
	}
	if len(bounces) != 1 || bounces[0].Campaign == nil || !json.Valid(*bounces[0].Campaign) {
		t.Fatalf("bounces=%#v", bounces)
	}
}

func TestSQLiteGetCampaignIncludesRelationships(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/campaign.db"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err := os.ReadFile("../../schema-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}

	var listID int
	if err = db.Get(&listID, "INSERT INTO lists(uuid,name,type,optin,status,tags) VALUES(?,?,?,?,?,?) RETURNING id",
		uuid.Must(uuid.NewV4()), "List", "public", "single", "active", "{}"); err != nil {
		t.Fatal(err)
	}
	var campaignID int
	if err = db.Get(&campaignID, `INSERT INTO campaigns
		(uuid,type,name,subject,from_email,body,content_type,messenger,archive,archive_meta)
		VALUES(?,?,?,?,?,?,?,?,?,?) RETURNING id`,
		uuid.Must(uuid.NewV4()), "regular", "Campaign", "Subject", "sender@example.com",
		"Body", "richtext", "email", false, dbtypes.RawJSON(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO campaign_lists(campaign_id,list_id,list_name) VALUES(?,?,?)",
		campaignID, listID, "List"); err != nil {
		t.Fatal(err)
	}

	queriesFile, err := os.ReadFile("../../queries-sqlite/campaigns.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(queriesFile)
	if err != nil {
		t.Fatal(err)
	}
	var fetched models.Campaign
	if err = db.Get(&fetched, queries["get-campaign"].Query, campaignID, nil, "", "default"); err != nil {
		t.Fatal(err)
	}
	if len(fetched.Lists) == 0 || len(fetched.Media) == 0 {
		t.Fatalf("campaign relationships missing: lists=%s media=%s", fetched.Lists, fetched.Media)
	}

	var stats struct {
		Lists string `db:"lists"`
		Media string `db:"media"`
	}
	if err = db.Get(&stats, queries["get-campaign-stats"].Query, pq.Array([]int{campaignID})); err != nil {
		t.Fatal(err)
	}
	if stats.Lists == "[]" || stats.Media != "[]" {
		t.Fatalf("campaign relationships = lists=%q media=%q", stats.Lists, stats.Media)
	}
}
