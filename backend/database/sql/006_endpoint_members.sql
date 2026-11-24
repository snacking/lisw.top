CREATE TABLE IF NOT EXISTS lisw_endpoint_members (
 endpoint_id VARCHAR(64) NOT NULL,
 user_id VARCHAR(64) NOT NULL,
 role ENUM('admin','user') NOT NULL,
 created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(endpoint_id,user_id),
 INDEX idx_lisw_endpoint_members_user(user_id,role),
 CONSTRAINT fk_lisw_em_endpoint FOREIGN KEY(endpoint_id) REFERENCES lisw_endpoints(id) ON DELETE CASCADE,
 CONSTRAINT fk_lisw_em_user FOREIGN KEY(user_id) REFERENCES lisw_users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
