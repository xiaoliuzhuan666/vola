ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_type VARCHAR(16) NOT NULL DEFAULT 'person';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_account_type_check'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_account_type_check
            CHECK (account_type IN ('person', 'team_hub')) NOT VALID;
        ALTER TABLE users
            VALIDATE CONSTRAINT users_account_type_check;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(256) NOT NULL,
    description TEXT DEFAULT '',
    hub_user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_team_members_user
    ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_team_role
    ON team_members(team_id, role);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'update_teams_updated_at'
    ) THEN
        CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'update_team_members_updated_at'
    ) THEN
        CREATE TRIGGER update_team_members_updated_at BEFORE UPDATE ON team_members
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;
