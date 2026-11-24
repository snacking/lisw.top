INSERT INTO lisw_services (id,endpoint_id,runtime_id,kind,name,status,version,revision,config,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE endpoint_id=VALUES(endpoint_id),runtime_id=VALUES(runtime_id),kind=VALUES(kind),name=VALUES(name),status=VALUES(status),version=VALUES(version),revision=VALUES(revision),config=VALUES(config),updated_at=VALUES(updated_at);
