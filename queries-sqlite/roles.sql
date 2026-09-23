-- name: get-user-roles
WITH list_perms AS (
 SELECT r.parent_id, json_group_array(json_object('id',r.list_id,'name',l.name,'permissions',json(listmonk_array(r.permissions)))) list_permissions
 FROM roles r LEFT JOIN lists l ON l.id=r.list_id WHERE r.parent_id IS NOT NULL GROUP BY r.parent_id)
SELECT r.*, CAST(COALESCE(p.list_permissions,'[]') AS BLOB) list_permissions FROM roles r
LEFT JOIN list_perms p ON p.parent_id=r.id
WHERE r.type='user' AND r.parent_id IS NULL AND ($1=0 OR r.id=$1) ORDER BY r.created_at;

-- name: get-list-roles
WITH list_perms AS (
 SELECT r.parent_id, json_group_array(json_object('id',r.list_id,'name',l.name,'permissions',json(listmonk_array(r.permissions)))) list_permissions
 FROM roles r LEFT JOIN lists l ON l.id=r.list_id WHERE r.parent_id IS NOT NULL GROUP BY r.parent_id)
SELECT r.*, CAST(COALESCE(p.list_permissions,'[]') AS BLOB) list_permissions FROM roles r
LEFT JOIN list_perms p ON p.parent_id=r.id WHERE r.type='list' AND r.parent_id IS NULL ORDER BY r.created_at;

-- name: create-role
INSERT INTO roles(name,type,permissions,created_at,updated_at) VALUES($1,$2,$3,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) RETURNING *;

-- name: upsert-list-permissions
INSERT INTO roles(parent_id,list_id,permissions,type)
SELECT $1, ids.value, listmonk_pg_array(json_extract(listmonk_array($3), '$['||ids.key||']')), 'list'
FROM json_each(listmonk_array($2)) ids
WHERE 1
ON CONFLICT(parent_id,list_id) DO UPDATE SET permissions=excluded.permissions;

-- name: delete-list-permission
DELETE FROM roles WHERE parent_id=$1 AND list_id=$2;

-- name: update-role
UPDATE roles SET name=$2,permissions=$3,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND parent_id IS NULL RETURNING *;

-- name: delete-role
DELETE FROM roles WHERE id=$1;
