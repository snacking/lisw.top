INSERT INTO lisw_endpoints (id,name,address,address_type,region,provider,owner_user_id) VALUES (?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE name=VALUES(name),address=VALUES(address),address_type=VALUES(address_type),region=VALUES(region),provider=VALUES(provider),owner_user_id=VALUES(owner_user_id);
