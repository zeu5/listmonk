-- name: create-user
INSERT INTO users(username,password_login,password,email,name,type,user_role_id,list_role_id,status)
VALUES($1,$2,CASE WHEN $6!='api' AND $2 AND $3!='' THEN listmonk_password($3,'') WHEN $6='api' THEN $3 ELSE NULL END,
 $4,$5,$6,(SELECT id FROM roles WHERE id=$7 AND type='user'),(SELECT id FROM roles WHERE id=$8 AND type='list'),$9)
RETURNING id;

-- name: update-user
UPDATE users SET username=CASE WHEN $2!='' THEN $2 ELSE username END,
 password_login=$3,
 password=CASE WHEN users.type='api' AND ($7='' OR $7='api') THEN password
   WHEN $3=1 THEN CASE WHEN $4!='' THEN listmonk_password($4,'') ELSE password END ELSE NULL END,
 email=CASE WHEN $5!='' THEN $5 ELSE email END,
 name=CASE WHEN $6!='' THEN $6 ELSE name END,
 type=CASE WHEN $7!='' THEN $7 ELSE type END,
 user_role_id=CASE WHEN $8!=0 THEN (SELECT id FROM roles WHERE id=$8 AND type='user') ELSE user_role_id END,
 list_role_id=CASE WHEN $9<0 THEN NULL WHEN $9>0 THEN (SELECT id FROM roles WHERE id=$9 AND type='list') ELSE list_role_id END,
 status=CASE WHEN $10!='' THEN $10 ELSE status END, updated_at=CURRENT_TIMESTAMP
WHERE id=$1 AND ((SELECT COUNT(*) FROM users WHERE id!=$1 AND status='enabled' AND type='user' AND user_role_id=1)>0 OR ($8=1 AND $10='enabled'));

-- name: delete-users
DELETE FROM users WHERE id IN (SELECT value FROM json_each(listmonk_array($1)))
AND (SELECT COUNT(*) FROM users WHERE id NOT IN (SELECT value FROM json_each(listmonk_array($1))) AND user_role_id=1 AND type='user' AND status='enabled')>0;

-- name: get-users
WITH lp AS (
 SELECT parent.id list_role_id,
 json_group_array(json_object('id',COALESCE(child.list_id,parent.list_id),'name',COALESCE(l.name,pl.name),'permissions',json(listmonk_array(COALESCE(child.permissions,parent.permissions))))) list_role_perms
 FROM roles parent LEFT JOIN roles child ON child.parent_id=parent.id AND child.type='list'
 LEFT JOIN lists l ON l.id=child.list_id LEFT JOIN lists pl ON pl.id=parent.list_id
 WHERE parent.type='list' AND parent.parent_id IS NULL GROUP BY parent.id)
SELECT users.*, ur.id user_role_id, ur.name user_role_name, ur.permissions user_role_permissions,
 lr.id list_role_id, lr.name list_role_name, CAST(lp.list_role_perms AS BLOB) list_role_perms
FROM users LEFT JOIN roles ur ON ur.id=users.user_role_id
LEFT JOIN roles lr ON lr.id=users.list_role_id LEFT JOIN lp ON lp.list_role_id=lr.id
ORDER BY users.created_at;

-- name: get-user
WITH sel AS (SELECT * FROM users WHERE CASE WHEN $1!=0 THEN id=$1 WHEN $2!='' THEN username=$2 WHEN $3!='' THEN email=$3 END)
SELECT sel.*, ur.id user_role_id, ur.name user_role_name, ur.permissions user_role_permissions,
 lr.id list_role_id, lr.name list_role_name,
 CAST((SELECT json_group_array(json_object('id',COALESCE(cr.list_id,lr.list_id),'name',COALESCE(cl.name,ll.name),'permissions',json(listmonk_array(COALESCE(cr.permissions,lr.permissions)))))
  FROM roles cr LEFT JOIN lists cl ON cl.id=cr.list_id LEFT JOIN lists ll ON ll.id=lr.list_id WHERE cr.parent_id=lr.id AND cr.type='list') AS BLOB) list_role_perms
FROM sel LEFT JOIN roles ur ON ur.id=sel.user_role_id AND ur.type='user'
LEFT JOIN roles lr ON lr.id=sel.list_role_id AND lr.type='list';

-- name: get-api-tokens
SELECT username,password FROM users WHERE status='enabled' AND type='api';

-- name: login-user
UPDATE users SET loggedin_at=CURRENT_TIMESTAMP
WHERE id=(SELECT id FROM users WHERE username=$1 AND status!='disabled' AND password_login=1 AND listmonk_password($2,password)=password)
RETURNING *, (SELECT name FROM roles WHERE id=users.user_role_id) role_name,
 (SELECT permissions FROM roles WHERE id=users.user_role_id) permissions;

-- name: update-user-profile
UPDATE users SET name=$2, email=CASE WHEN password_login THEN $3 ELSE email END,
 password=CASE WHEN $4=1 THEN CASE WHEN $5!='' THEN listmonk_password($5,'') ELSE password END ELSE NULL END WHERE id=$1;

-- name: update-user-login
UPDATE users SET loggedin_at=CURRENT_TIMESTAMP,avatar=CASE WHEN $2!='' THEN $2 ELSE avatar END WHERE id=$1;

-- name: set-user-twofa
UPDATE users SET twofa_type=$2,twofa_key=$3,updated_at=CURRENT_TIMESTAMP WHERE id=$1;

-- name: delete-user-sessions
DELETE FROM sessions WHERE json_extract(data,'$.user_id')=$1 AND ($2='' OR id!=$2);
