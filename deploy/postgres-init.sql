-- Local dev: one Postgres instance hosting one logical database per service
-- (mirrors DB-per-service in prod, agent_plan §3/§5.2). Each service connects only
-- to its own database. RLS policies are added by each service's migrations.
CREATE DATABASE identity;
CREATE DATABASE tenant;
CREATE DATABASE student;
CREATE DATABASE staff;
CREATE DATABASE academic;
CREATE DATABASE attendance;
CREATE DATABASE assessment;
CREATE DATABASE report;
CREATE DATABASE fees;
CREATE DATABASE payment;
CREATE DATABASE notification;
CREATE DATABASE website;
CREATE DATABASE file;
CREATE DATABASE analytics;
CREATE DATABASE billing;
CREATE DATABASE cbt;
CREATE DATABASE audit;
CREATE DATABASE ai;
