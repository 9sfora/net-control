-- Development-only role. Production provisioning must create an equivalent
-- non-owner role with a secret supplied by the deployment environment.
CREATE ROLE netcontrol_app LOGIN PASSWORD 'netcontrol_app';
GRANT CONNECT ON DATABASE netcontrol TO netcontrol_app;
GRANT USAGE ON SCHEMA public TO netcontrol_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO netcontrol_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO netcontrol_app;
ALTER DEFAULT PRIVILEGES FOR ROLE netcontrol IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO netcontrol_app;
ALTER DEFAULT PRIVILEGES FOR ROLE netcontrol IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO netcontrol_app;
