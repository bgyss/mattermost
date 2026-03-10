-- Workgroups table: virtual company departments
CREATE TABLE IF NOT EXISTS Workgroups (
    Id VARCHAR(26) PRIMARY KEY,
    TeamId VARCHAR(26) NOT NULL,
    Name VARCHAR(64) NOT NULL,
    DisplayName VARCHAR(255) NOT NULL,
    Type VARCHAR(32) NOT NULL DEFAULT 'department',
    HeadAgentId VARCHAR(26) DEFAULT '',
    Status VARCHAR(32) NOT NULL DEFAULT 'provisioning',
    ChannelIds JSONB NOT NULL DEFAULT '{}',
    CreateAt BIGINT NOT NULL,
    UpdateAt BIGINT NOT NULL,
    DeleteAt BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_workgroups_team_id ON Workgroups(TeamId);
CREATE INDEX IF NOT EXISTS idx_workgroups_name ON Workgroups(Name);
CREATE INDEX IF NOT EXISTS idx_workgroups_delete_at ON Workgroups(DeleteAt);

-- AgentDefinitions table: specification of each agent
CREATE TABLE IF NOT EXISTS AgentDefinitions (
    Id VARCHAR(26) PRIMARY KEY,
    WorkgroupId VARCHAR(26) DEFAULT '',
    Role VARCHAR(32) NOT NULL DEFAULT 'specialist',
    BotUserId VARCHAR(26) DEFAULT '',
    DisplayName VARCHAR(255) NOT NULL,
    SystemPrompt TEXT NOT NULL DEFAULT '',
    LLMServiceId VARCHAR(64) DEFAULT '',
    ModelId VARCHAR(128) DEFAULT 'claude-sonnet-4-6',
    ModelParameters JSONB NOT NULL DEFAULT '{}',
    Tools JSONB NOT NULL DEFAULT '[]',
    Capabilities JSONB NOT NULL DEFAULT '[]',
    MaxConcurrency INT NOT NULL DEFAULT 5,
    PoolSize INT NOT NULL DEFAULT 1,
    MemoryConfig JSONB NOT NULL DEFAULT '{}',
    OwnerUserId VARCHAR(26) DEFAULT '',
    CreateAt BIGINT NOT NULL,
    UpdateAt BIGINT NOT NULL,
    DeleteAt BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_agent_definitions_workgroup_id ON AgentDefinitions(WorkgroupId);
CREATE INDEX IF NOT EXISTS idx_agent_definitions_bot_user_id ON AgentDefinitions(BotUserId);
CREATE INDEX IF NOT EXISTS idx_agent_definitions_role ON AgentDefinitions(Role);
CREATE INDEX IF NOT EXISTS idx_agent_definitions_delete_at ON AgentDefinitions(DeleteAt);

-- AgentTasks table: atomic units of work
CREATE TABLE IF NOT EXISTS AgentTasks (
    Id VARCHAR(26) PRIMARY KEY,
    AgentId VARCHAR(26) NOT NULL,
    ParentTaskId VARCHAR(26) DEFAULT '',
    RootTaskId VARCHAR(26) NOT NULL,
    ChannelId VARCHAR(26) NOT NULL,
    ThreadRootPostId VARCHAR(26) DEFAULT '',
    RequestPostId VARCHAR(26) DEFAULT '',
    ResponsePostId VARCHAR(26) DEFAULT '',
    Input JSONB NOT NULL DEFAULT '{}',
    Output JSONB NOT NULL DEFAULT '{}',
    Status VARCHAR(32) NOT NULL DEFAULT 'pending',
    Priority INT NOT NULL DEFAULT 50,
    DelegationChain JSONB NOT NULL DEFAULT '[]',
    TokensUsed INT NOT NULL DEFAULT 0,
    LatencyMs BIGINT NOT NULL DEFAULT 0,
    ErrorMsg TEXT DEFAULT '',
    CreateAt BIGINT NOT NULL,
    UpdateAt BIGINT NOT NULL,
    ClaimedAt BIGINT NOT NULL DEFAULT 0,
    CompleteAt BIGINT NOT NULL DEFAULT 0,
    ClaimedByServer VARCHAR(26) DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_agent_tasks_agent_id ON AgentTasks(AgentId);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_status ON AgentTasks(Status);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_root_task_id ON AgentTasks(RootTaskId);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_parent_task_id ON AgentTasks(ParentTaskId);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_channel_id ON AgentTasks(ChannelId);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_create_at ON AgentTasks(CreateAt);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_agent_status ON AgentTasks(AgentId, Status);

-- AgentMemory table: persistent agent memory (KV + semantic)
CREATE TABLE IF NOT EXISTS AgentMemory (
    Id VARCHAR(26) PRIMARY KEY,
    AgentId VARCHAR(26) NOT NULL,
    Scope VARCHAR(32) NOT NULL DEFAULT 'global',
    ScopeId VARCHAR(26) DEFAULT '',
    Key VARCHAR(255) NOT NULL,
    ValueText TEXT NOT NULL DEFAULT '',
    ExpireAt BIGINT NOT NULL DEFAULT 0,
    CreateAt BIGINT NOT NULL,
    UpdateAt BIGINT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_memory_agent_scope_key ON AgentMemory(AgentId, Scope, ScopeId, Key);
CREATE INDEX IF NOT EXISTS idx_agent_memory_agent_id ON AgentMemory(AgentId);
CREATE INDEX IF NOT EXISTS idx_agent_memory_expire_at ON AgentMemory(ExpireAt) WHERE ExpireAt > 0;

-- AgentTaskEvents table: observability stream
CREATE TABLE IF NOT EXISTS AgentTaskEvents (
    Id VARCHAR(26) PRIMARY KEY,
    TaskId VARCHAR(26) NOT NULL,
    AgentId VARCHAR(26) NOT NULL,
    EventType VARCHAR(64) NOT NULL,
    Payload JSONB NOT NULL DEFAULT '{}',
    CreateAt BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_task_events_task_id ON AgentTaskEvents(TaskId);
CREATE INDEX IF NOT EXISTS idx_agent_task_events_agent_id ON AgentTaskEvents(AgentId);
CREATE INDEX IF NOT EXISTS idx_agent_task_events_create_at ON AgentTaskEvents(CreateAt);
