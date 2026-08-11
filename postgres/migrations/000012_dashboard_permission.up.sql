INSERT INTO public.permissions (code, description)
VALUES ('acs.dashboard.read', 'View ACS operational dashboard');

INSERT INTO public.role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM public.roles r
CROSS JOIN public.permissions p
WHERE r.name = 'Administrator' AND p.code = 'acs.dashboard.read'
ON CONFLICT DO NOTHING;
