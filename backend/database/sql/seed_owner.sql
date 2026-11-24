INSERT INTO lisw_users (id,username,display_name,github_username,platform_role,is_demo)
VALUES ('admin','snacking','Snacking','snacking','admin',FALSE)
ON DUPLICATE KEY UPDATE username='snacking', display_name='Snacking', github_username='snacking', platform_role='admin', is_demo=FALSE;
