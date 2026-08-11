DROP TRIGGER IF EXISTS tenants_set_updated_at ON public.tenants;
DROP TABLE IF EXISTS public.tenants;
DROP FUNCTION IF EXISTS public.set_updated_at();
