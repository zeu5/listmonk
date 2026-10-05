package dbops

import (
	"os"
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
	if err = db.Get(&created, "SELECT campaigns.*, '' AS template_body FROM campaigns WHERE id=?", campaignID); err != nil {
		t.Fatal(err)
	}
	if created.Body == "" {
		t.Fatal("expected campaign body to fall back to default template")
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
