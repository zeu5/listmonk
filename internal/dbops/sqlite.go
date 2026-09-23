// Package dbops supplies backend-specific implementations for operations that
// cannot be represented by one portable SQL statement.
package dbops

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/listmonk/models"
)

// SQLite returns operation overrides keyed by goyesql query name.
func SQLite(db *sqlx.DB) map[string]models.Statement {
	ops := map[string]models.Statement{
		"upsert-list-permissions": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			if _, err = tx.Exec("DELETE FROM roles WHERE parent_id=? AND list_id NOT IN (SELECT value FROM json_each(listmonk_array(?)))", args[0], args[1]); err != nil {
				return nil, err
			}
			res, err := tx.Exec(`INSERT INTO roles(parent_id,list_id,permissions,type) SELECT ?,ids.value,listmonk_pg_array(json_extract(listmonk_array(?),'$['||ids.key||']')),'list' FROM json_each(listmonk_array(?)) ids WHERE 1 ON CONFLICT(parent_id,list_id) DO UPDATE SET permissions=excluded.permissions`, args[0], args[2], args[1])
			if err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return res, nil
		}},
		"create-campaign": &models.FuncStatement{GetFunc: func(dest any, args ...any) error {
			if len(args) != 21 {
				return fmt.Errorf("create-campaign: got %d arguments, want 21", len(args))
			}
			tx, err := db.Beginx()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			var tpl struct {
				ID     sql.NullInt64  `db:"id"`
				Body   string         `db:"body"`
				Source sql.NullString `db:"body_source"`
			}
			if args[13] != nil {
				_ = tx.Get(&tpl, "SELECT CASE WHEN type='campaign_visual' THEN NULL ELSE id END id,CASE WHEN type='campaign_visual' THEN body ELSE '' END body,body_source FROM templates WHERE id=?", args[13])
			} else {
				_ = tx.Get(&tpl, "SELECT id,'' body,body_source FROM templates WHERE is_default=1 AND ?!='visual' LIMIT 1", args[7])
			}
			body := fmt.Sprint(args[5])
			if body == "" {
				body = tpl.Body
			}
			source := args[20]
			if source == nil {
				source = tpl.Source
			}
			err = tx.Get(dest, `INSERT INTO campaigns(uuid,type,name,subject,from_email,body,altbody,content_type,send_at,headers,attribs,tags,messenger,template_id,to_send,max_subscriber_id,archive,archive_slug,archive_template_id,archive_meta,body_source) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,0,?,?,?,?,?) RETURNING id`, args[0], args[1], args[2], args[3], args[4], body, args[6], args[7], args[8], args[9], args[10], args[11], args[12], tpl.ID, args[15], args[16], args[17], args[18], source)
			if err != nil {
				return err
			}
			var id int
			if err = tx.Get(&id, "SELECT id FROM campaigns WHERE uuid=?", args[0]); err != nil {
				return err
			}
			if _, err = tx.Exec("INSERT INTO campaign_lists(campaign_id,list_id,list_name) SELECT ?,id,name FROM lists WHERE id IN (SELECT value FROM json_each(listmonk_array(?)))", id, args[14]); err != nil {
				return err
			}
			if _, err = tx.Exec("INSERT INTO campaign_media(campaign_id,media_id,filename) SELECT ?,id,filename FROM media WHERE id IN (SELECT value FROM json_each(listmonk_array(?)))", id, args[19]); err != nil {
				return err
			}
			return tx.Commit()
		}},
		"update-campaign": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			res, err := tx.Exec(`UPDATE campaigns SET name=?,subject=?,from_email=?,body=?,altbody=NULLIF(?,''),content_type=?,send_at=?,status=CASE WHEN status='scheduled' AND ? IS NULL THEN 'draft' ELSE status END,headers=?,attribs=?,tags=?,messenger=?,template_id=CASE WHEN ?='visual' THEN NULL ELSE ? END,archive=?,archive_slug=?,archive_template_id=CASE WHEN ?='visual' THEN NULL ELSE ? END,archive_meta=?,body_source=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[7], args[8], args[9], args[10], args[11], args[6], args[12], args[14], args[15], args[6], args[16], args[17], args[19], args[0])
			if err != nil {
				return nil, err
			}
			if _, err = tx.Exec("DELETE FROM campaign_lists WHERE campaign_id=? AND list_id NOT IN (SELECT value FROM json_each(listmonk_array(?)))", args[0], args[13]); err != nil {
				return nil, err
			}
			if _, err = tx.Exec("INSERT INTO campaign_lists(campaign_id,list_id,list_name) SELECT ?,id,name FROM lists WHERE id IN (SELECT value FROM json_each(listmonk_array(?))) AND 1 ON CONFLICT(campaign_id,list_id) DO UPDATE SET list_name=excluded.list_name", args[0], args[13]); err != nil {
				return nil, err
			}
			if _, err = tx.Exec("DELETE FROM campaign_media WHERE campaign_id=? AND (media_id IS NULL OR media_id NOT IN (SELECT value FROM json_each(listmonk_array(?))))", args[0], args[18]); err != nil {
				return nil, err
			}
			if _, err = tx.Exec("INSERT INTO campaign_media(campaign_id,media_id,filename) SELECT ?,id,filename FROM media WHERE id IN (SELECT value FROM json_each(listmonk_array(?))) AND 1 ON CONFLICT(campaign_id,media_id) DO NOTHING", args[0], args[18]); err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return res, nil
		}},
		"next-campaigns": &models.FuncStatement{SelectFunc: func(dest any, args ...any) error {
			tx, err := db.Beginx()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			_, err = tx.Exec(`UPDATE campaigns SET sent=sent+COALESCE((SELECT counts.value FROM json_each(listmonk_array(?)) ids JOIN json_each(listmonk_array(?)) counts ON counts.key=ids.key WHERE CAST(ids.value AS INTEGER)=campaigns.id),0) WHERE id IN (SELECT value FROM json_each(listmonk_array(?)))`, args[0], args[1], args[0])
			if err != nil {
				return err
			}
			_, err = tx.Exec(`UPDATE campaigns SET to_send=(SELECT COUNT(DISTINCT sl.subscriber_id) FROM campaign_lists cl JOIN lists l ON l.id=cl.list_id JOIN subscriber_lists sl ON sl.list_id=l.id JOIN subscribers s ON s.id=sl.subscriber_id WHERE cl.campaign_id=campaigns.id AND s.status!='blocklisted' AND (CASE WHEN campaigns.type='optin' THEN sl.status='unconfirmed' AND l.optin='double' WHEN l.optin='double' THEN sl.status='confirmed' ELSE sl.status!='unsubscribed' END)),max_subscriber_id=COALESCE((SELECT MAX(sl.subscriber_id) FROM campaign_lists cl JOIN subscriber_lists sl ON sl.list_id=cl.list_id WHERE cl.campaign_id=campaigns.id),0),status='running',started_at=COALESCE(started_at,CURRENT_TIMESTAMP) WHERE (status='running' OR (status='scheduled' AND CURRENT_TIMESTAMP>=send_at)) AND id NOT IN (SELECT value FROM json_each(listmonk_array(?)))`, args[0])
			if err != nil {
				return err
			}
			err = tx.Select(dest, `SELECT c.*,COALESCE(t.body,(SELECT body FROM templates WHERE is_default=1 LIMIT 1),'') template_body,COALESCE((SELECT '{'||group_concat(media_id)||'}' FROM campaign_media WHERE campaign_id=c.id AND media_id IS NOT NULL),'{}') media_id FROM campaigns c LEFT JOIN templates t ON t.id=c.template_id WHERE c.status='running' AND c.id NOT IN (SELECT value FROM json_each(listmonk_array(?)))`, args[0])
			if err != nil {
				return err
			}
			return tx.Commit()
		}},
		"next-campaign-subscribers": &models.FuncStatement{SelectFunc: func(dest any, args ...any) error {
			tx, err := db.Beginx()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			err = tx.Select(dest, `WITH camp_lists AS (SELECT l.id,l.optin FROM lists l JOIN campaign_lists cl ON cl.list_id=l.id WHERE cl.campaign_id=?) SELECT DISTINCT s.* FROM subscriber_lists sl JOIN camp_lists cl ON cl.id=sl.list_id JOIN subscribers s ON s.id=sl.subscriber_id WHERE sl.list_id IN (SELECT value FROM json_each(listmonk_array(?))) AND s.id>? AND s.id<=? AND s.status!='blocklisted' AND ((?='optin' AND sl.status='unconfirmed' AND cl.optin='double') OR (?!='optin' AND ((cl.optin='double' AND sl.status='confirmed') OR (cl.optin!='double' AND sl.status!='unsubscribed')))) ORDER BY s.id LIMIT ?`, args[0], args[4], args[2], args[3], args[1], args[1], args[5])
			if err != nil {
				return err
			}
			subs, ok := dest.(*[]models.Subscriber)
			if ok && len(*subs) > 0 {
				_, err = tx.Exec("UPDATE campaigns SET last_subscriber_id=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", (*subs)[len(*subs)-1].ID, args[0])
				if err != nil {
					return err
				}
			}
			return tx.Commit()
		}},
		"insert-subscriber": &models.FuncStatement{GetFunc: func(dest any, args ...any) error {
			if len(args) != 8 {
				return fmt.Errorf("insert-subscriber: got %d arguments, want 8", len(args))
			}
			tx, err := db.Beginx()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			if err = tx.Get(dest, "INSERT INTO subscribers(uuid,email,name,status,attribs) VALUES(?,?,?,?,?) RETURNING id", args[:5]...); err != nil {
				return err
			}
			status := args[7]
			if fmt.Sprint(args[3]) == "blocklisted" {
				status = "unsubscribed"
			}
			_, err = tx.Exec(`INSERT INTO subscriber_lists(subscriber_id,list_id,status)
			SELECT (SELECT id FROM subscribers WHERE email=?),id,? FROM lists WHERE
			(json_array_length(listmonk_array(?))>0 AND id IN (SELECT value FROM json_each(listmonk_array(?)))) OR
			(json_array_length(listmonk_array(?))=0 AND uuid IN (SELECT value FROM json_each(listmonk_array(?))))
			ON CONFLICT(subscriber_id,list_id) DO UPDATE SET status=excluded.status,updated_at=CURRENT_TIMESTAMP`, args[1], status, args[5], args[5], args[5], args[6])
			if err != nil {
				return err
			}
			return tx.Commit()
		}},
		"upsert-subscriber": &models.FuncStatement{GetFunc: func(dest any, args ...any) error {
			if len(args) != 8 {
				return fmt.Errorf("upsert-subscriber: got %d arguments, want 8", len(args))
			}
			tx, err := db.Beginx()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			err = tx.Get(dest, `INSERT INTO subscribers(uuid,email,name,attribs,status) VALUES(?,?,?,?,'enabled')
			ON CONFLICT(email) DO UPDATE SET name=CASE WHEN ? THEN excluded.name ELSE subscribers.name END,
			attribs=CASE WHEN ? THEN excluded.attribs ELSE subscribers.attribs END,updated_at=CURRENT_TIMESTAMP RETURNING uuid,id`, args[0], args[1], args[2], args[3], args[6], args[6])
			if err != nil {
				return err
			}
			_, err = tx.Exec(`INSERT INTO subscriber_lists(subscriber_id,list_id,status)
			SELECT (SELECT id FROM subscribers WHERE email=?),value,CASE WHEN (SELECT status FROM subscribers WHERE email=?)='blocklisted' THEN 'unsubscribed' ELSE ? END
			FROM json_each(listmonk_array(?)) WHERE 1 ON CONFLICT(subscriber_id,list_id) DO UPDATE SET
			status=CASE WHEN ? THEN excluded.status ELSE subscriber_lists.status END,updated_at=CURRENT_TIMESTAMP`, args[1], args[1], args[5], args[4], args[7])
			if err != nil {
				return err
			}
			return tx.Commit()
		}},
		"upsert-blocklist-subscriber": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			res, err := tx.Exec(`INSERT INTO subscribers(uuid,email,name,attribs,status) VALUES(?,?,?,?,'blocklisted') ON CONFLICT(email) DO UPDATE SET status='blocklisted',updated_at=CURRENT_TIMESTAMP`, args...)
			if err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return res, nil
		}},
		"update-subscriber-with-lists": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			res, err := tx.Exec(`UPDATE subscribers SET email=CASE WHEN ?!='' THEN ? ELSE email END,name=CASE WHEN ?!='' THEN ? ELSE name END,status=CASE WHEN ?!='' THEN ? ELSE status END,attribs=CASE WHEN ?!='' THEN ? ELSE attribs END,updated_at=CURRENT_TIMESTAMP WHERE id=?`, args[1], args[1], args[2], args[2], args[3], args[3], args[4], args[4], args[0])
			if err != nil {
				return nil, err
			}
			if del, _ := args[8].(bool); del {
				_, err = tx.Exec(`DELETE FROM subscriber_lists WHERE subscriber_id=? AND list_id NOT IN (SELECT id FROM lists WHERE id IN (SELECT value FROM json_each(listmonk_array(?))) OR uuid IN (SELECT value FROM json_each(listmonk_array(?)))) AND (json_array_length(listmonk_array(?))=0 OR list_id IN (SELECT value FROM json_each(listmonk_array(?))))`, args[0], args[5], args[6], args[9], args[9])
				if err != nil {
					return nil, err
				}
			}
			status := args[7]
			if fmt.Sprint(args[3]) == "blocklisted" {
				status = "unsubscribed"
			}
			_, err = tx.Exec(`INSERT INTO subscriber_lists(subscriber_id,list_id,status) SELECT ?,id,? FROM lists WHERE id IN (SELECT value FROM json_each(listmonk_array(?))) OR uuid IN (SELECT value FROM json_each(listmonk_array(?))) ON CONFLICT(subscriber_id,list_id) DO UPDATE SET status=CASE WHEN subscriber_lists.status='confirmed' THEN 'confirmed' WHEN ? THEN excluded.status WHEN subscriber_lists.status='unsubscribed' THEN 'unsubscribed' ELSE excluded.status END`, args[0], status, args[5], args[6], args[10])
			if err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return res, nil
		}},
		"unsubscribe-by-campaign": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			if block, _ := args[2].(bool); block {
				if _, err = tx.Exec("UPDATE subscribers SET status='blocklisted',updated_at=CURRENT_TIMESTAMP WHERE uuid=?", args[1]); err != nil {
					return nil, err
				}
			}
			res, err := tx.Exec(`UPDATE subscriber_lists SET status='unsubscribed',updated_at=CURRENT_TIMESTAMP WHERE subscriber_id=(SELECT id FROM subscribers WHERE uuid=?) AND (? OR list_id IN (SELECT cl.list_id FROM campaign_lists cl JOIN campaigns c ON c.id=cl.campaign_id WHERE c.uuid=?))`, args[1], args[2], args[0])
			if err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return res, nil
		}},
		"record-bounce": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			if len(args) != 9 {
				return nil, fmt.Errorf("record-bounce: got %d arguments, want 9", len(args))
			}
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			var sub struct {
				ID     int    `db:"id"`
				Status string `db:"status"`
			}
			if fmt.Sprint(args[0]) != "" {
				err = tx.Get(&sub, "SELECT id,status FROM subscribers WHERE uuid=?", args[0])
			} else {
				err = tx.Get(&sub, "SELECT id,status FROM subscribers WHERE email=?", args[1])
			}
			if err != nil {
				return nil, err
			}
			var count int
			if err = tx.Get(&count, "SELECT count(*)+1 FROM bounces WHERE subscriber_id=? AND type=?", sub.ID, args[3]); err != nil {
				return nil, err
			}
			threshold, ok := asInt(args[7])
			if !ok {
				return nil, fmt.Errorf("invalid bounce threshold %T", args[7])
			}
			action := fmt.Sprint(args[8])
			if count >= threshold {
				switch action {
				case "blocklist":
					_, err = tx.Exec("UPDATE subscribers SET status='blocklisted',updated_at=CURRENT_TIMESTAMP WHERE id=?", sub.ID)
				case "unsubscribe":
					_, err = tx.Exec("UPDATE subscriber_lists SET status='unsubscribed',updated_at=CURRENT_TIMESTAMP WHERE subscriber_id=?", sub.ID)
				case "delete":
					_, err = tx.Exec("DELETE FROM subscribers WHERE id=?", sub.ID)
				}
				if err != nil {
					return nil, err
				}
			}
			var campaignID any
			if fmt.Sprint(args[2]) != "" {
				var id int
				if e := tx.Get(&id, "SELECT id FROM campaigns WHERE uuid=?", args[2]); e == nil {
					campaignID = id
				}
			}
			var result sql.Result
			if sub.Status != "blocklisted" && count <= threshold {
				result, err = tx.Exec("INSERT INTO bounces(subscriber_id,campaign_id,type,source,meta,created_at) VALUES(?,?,?,?,?,?)", sub.ID, campaignID, args[3], args[4], args[5], args[6])
				if err != nil {
					return nil, err
				}
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return result, nil
		}},
		"blocklist-bounced-subscribers": &models.FuncStatement{ExecFunc: func(args ...any) (sql.Result, error) {
			tx, err := db.Beginx()
			if err != nil {
				return nil, err
			}
			defer tx.Rollback()
			res, err := tx.Exec("UPDATE subscribers SET status='blocklisted',updated_at=CURRENT_TIMESTAMP WHERE id IN (SELECT subscriber_id FROM bounces)")
			if err != nil {
				return nil, err
			}
			if _, err = tx.Exec("UPDATE subscriber_lists SET status='unsubscribed',updated_at=CURRENT_TIMESTAMP WHERE subscriber_id IN (SELECT subscriber_id FROM bounces)"); err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return res, nil
		}},
	}
	// The importer and installer use Exec for the same upsert operation whose
	// API path uses Get. Both execute the identical transaction.
	upsert := ops["upsert-subscriber"].(*models.FuncStatement)
	upsert.ExecFunc = func(args ...any) (sql.Result, error) {
		var out struct {
			UUID string `db:"uuid"`
			ID   int    `db:"id"`
		}
		return nil, upsert.GetFunc(&out, args...)
	}
	createCampaign := ops["create-campaign"].(*models.FuncStatement)
	createCampaign.ExecFunc = func(args ...any) (sql.Result, error) {
		var id int
		return nil, createCampaign.GetFunc(&id, args...)
	}
	return ops
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case int32:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
