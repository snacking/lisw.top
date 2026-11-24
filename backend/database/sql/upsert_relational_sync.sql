INSERT INTO lisw_relational_sync (id,source_version) VALUES ('workspace',?) ON DUPLICATE KEY UPDATE source_version=VALUES(source_version);
