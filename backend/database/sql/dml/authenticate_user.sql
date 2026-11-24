SELECT id,username,display_name,platform_role,password_hash FROM lisw_users WHERE (username=? OR email=?) AND status='active' AND is_demo=FALSE AND password_hash IS NOT NULL LIMIT 1;
