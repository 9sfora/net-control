INSERT INTO public.permissions (code, description) VALUES
    ('acs.command.read', 'View ACS commands and execution history'),
    ('acs.command.cancel', 'Cancel ACS commands before execution');

INSERT INTO public.role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM public.roles r
CROSS JOIN public.permissions p
WHERE r.name = 'Administrator'
  AND p.code IN ('acs.command.read', 'acs.command.cancel')
ON CONFLICT DO NOTHING;
