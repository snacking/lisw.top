CREATE TABLE IF NOT EXISTS lisw_services (
 id VARCHAR(64) NOT NULL PRIMARY KEY,
 endpoint_id VARCHAR(64) NOT NULL,
 runtime_id VARCHAR(64) NOT NULL,
 kind VARCHAR(64) NOT NULL,
 name VARCHAR(128) NOT NULL,
 status VARCHAR(32) NOT NULL,
 version VARCHAR(128) NOT NULL DEFAULT '',
 revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
 config JSON NOT NULL,
 control_mode ENUM('mock','managed') NOT NULL DEFAULT 'mock',
 created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
 INDEX idx_lisw_services_endpoint_kind(endpoint_id,kind),
 INDEX idx_lisw_services_runtime(runtime_id),
 CONSTRAINT fk_lisw_service_endpoint FOREIGN KEY(endpoint_id) REFERENCES lisw_endpoints(id) ON DELETE RESTRICT,
 CONSTRAINT fk_lisw_service_runtime FOREIGN KEY(runtime_id) REFERENCES lisw_runtimes(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
