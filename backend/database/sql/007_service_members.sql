CREATE TABLE IF NOT EXISTS lisw_service_members (
 service_id VARCHAR(64) NOT NULL,
 user_id VARCHAR(64) NOT NULL,
 role ENUM('admin','user') NOT NULL,
 created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 PRIMARY KEY(service_id,user_id),
 INDEX idx_lisw_service_members_user(user_id,role),
 CONSTRAINT fk_lisw_sm_service FOREIGN KEY(service_id) REFERENCES lisw_services(id) ON DELETE CASCADE,
 CONSTRAINT fk_lisw_sm_user FOREIGN KEY(user_id) REFERENCES lisw_users(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
