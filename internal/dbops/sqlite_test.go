package dbops

import (
	"os"
	"testing"

	"github.com/knadh/listmonk/internal/dbconn"
	"github.com/knadh/listmonk/models"
	"github.com/lib/pq"
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
	args := []any{"camp-uuid", "regular", "Campaign", "Subject", "sender@example.com", "Body", "", "richtext", nil, []byte(`[]`), []byte(`{}`), pq.StringArray{"tag"}, "email", nil, pq.Array([]int{listID}), false, "", nil, []byte(`{}`), pq.Array([]int{}), nil}
	if err = ops["create-campaign"].Get(&campaignID, args...); err != nil {
		t.Fatal(err)
	}
	var links int
	if err = db.Get(&links, "SELECT count(*) FROM campaign_lists WHERE campaign_id=? AND list_id=?", campaignID, listID); err != nil || links != 1 {
		t.Fatalf("campaign lists=%d err=%v", links, err)
	}
	var subscribers []models.Subscriber
	if err = ops["next-campaign-subscribers"].Select(&subscribers, campaignID, "regular", 0, sub.ID, pq.Array([]int{listID}), 100); err != nil {
		t.Fatal(err)
	}
	if len(subscribers) != 1 || subscribers[0].ID != sub.ID {
		t.Fatalf("subscribers=%#v", subscribers)
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
