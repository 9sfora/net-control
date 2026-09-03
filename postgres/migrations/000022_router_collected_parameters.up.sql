ALTER TABLE acs.routers
    ADD COLUMN collected_parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN last_sync_at TIMESTAMPTZ;

WITH latest_commands AS (
    SELECT DISTINCT ON (tenant_id,router_id)
           id,tenant_id,router_id,result,completed_at
    FROM acs.router_commands
    WHERE type='SYNC' AND status='COMPLETED'
    ORDER BY tenant_id,router_id,completed_at DESC
), collected AS (
    SELECT c.tenant_id,c.router_id,c.completed_at,
           COALESCE(jsonb_object_agg(parameter->>'name',parameter->'value'),'{}'::jsonb) AS parameters
    FROM latest_commands c
    CROSS JOIN LATERAL jsonb_array_elements(COALESCE(c.result->'parameters','[]'::jsonb)) parameter
    GROUP BY c.tenant_id,c.router_id,c.completed_at
)
UPDATE acs.routers r
SET collected_parameters=collected.parameters,last_sync_at=collected.completed_at
FROM collected
WHERE collected.tenant_id=r.tenant_id AND collected.router_id=r.id;
