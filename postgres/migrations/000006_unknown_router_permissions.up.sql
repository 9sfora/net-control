INSERT INTO public.permissions (code, description) VALUES
    ('acs.unknown_router.read', 'View unknown routers'),
    ('acs.unknown_router.configure', 'Configure unknown routers');

INSERT INTO public.role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM public.roles r
CROSS JOIN public.permissions p
WHERE r.name = 'Administrator'
  AND p.code IN ('acs.unknown_router.read', 'acs.unknown_router.configure')
ON CONFLICT DO NOTHING;
