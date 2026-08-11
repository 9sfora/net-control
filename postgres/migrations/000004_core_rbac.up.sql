CREATE TABLE public.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id),
    name TEXT NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT roles_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT roles_tenant_name_unique UNIQUE (tenant_id, name),
    CONSTRAINT roles_tenant_id_id_unique UNIQUE (tenant_id, id)
);

CREATE TABLE public.permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT permissions_code_format CHECK (code ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$')
);

CREATE TABLE public.user_roles (
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES public.users (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, role_id) REFERENCES public.roles (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE public.role_permissions (
    tenant_id UUID NOT NULL,
    role_id UUID NOT NULL,
    permission_id UUID NOT NULL REFERENCES public.permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (tenant_id, role_id) REFERENCES public.roles (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX roles_tenant_id_idx ON public.roles (tenant_id);
CREATE INDEX user_roles_tenant_id_idx ON public.user_roles (tenant_id);
CREATE INDEX role_permissions_tenant_id_idx ON public.role_permissions (tenant_id);

CREATE TRIGGER roles_set_updated_at
BEFORE UPDATE ON public.roles
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE public.roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.roles FORCE ROW LEVEL SECURITY;
CREATE POLICY roles_tenant_isolation ON public.roles
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE public.user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY user_roles_tenant_isolation ON public.user_roles
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE public.role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.role_permissions FORCE ROW LEVEL SECURITY;
CREATE POLICY role_permissions_tenant_isolation ON public.role_permissions
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

INSERT INTO public.permissions (code, description) VALUES
    ('acs.router.read', 'View routers'),
    ('acs.router.create', 'Create routers'),
    ('acs.router.update', 'Update routers'),
    ('acs.router.delete', 'Delete routers'),
    ('acs.router.command.reboot', 'Reboot routers'),
    ('acs.router.command.activate', 'Activate routers'),
    ('acs.router.command.block', 'Block routers'),
    ('acs.router.command.update_firmware', 'Update router firmware'),
    ('acs.firmware.read', 'View firmware'),
    ('acs.firmware.create', 'Create firmware'),
    ('acs.firmware.upload', 'Upload firmware files'),
    ('acs.blacklist.read', 'View blacklist'),
    ('acs.blacklist.create', 'Create blacklist entries'),
    ('acs.blacklist.delete', 'Delete blacklist entries'),
    ('system.users.read', 'View users'),
    ('system.users.manage', 'Manage users'),
    ('system.settings.read', 'View settings'),
    ('system.settings.manage', 'Manage settings');
