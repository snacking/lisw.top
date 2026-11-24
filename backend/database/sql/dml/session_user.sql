SELECT id,username,display_name,platform_role FROM lisw_users WHERE id=? AND status='active' AND is_demo=FALSE AND password_hash IS NOT NULL;
