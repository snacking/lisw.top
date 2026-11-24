INSERT INTO lisw_runtimes (id,name,location,host_address) VALUES (?,?,?,?)
ON DUPLICATE KEY UPDATE name=VALUES(name),location=VALUES(location),host_address=VALUES(host_address);
