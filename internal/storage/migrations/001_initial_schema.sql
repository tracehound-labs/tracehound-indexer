-- Tracehound Indexer - Initial Schema
-- 🐕 This is where we store the whale scents we've tracked

-- Create the main table for storing blockchain events
CREATE TABLE IF NOT EXISTS whale_scents (
    id BIGSERIAL PRIMARY KEY,
    tx_hash TEXT UNIQUE NOT NULL,
    block_height BIGINT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('trade', 'position', 'liquidation')),
    timestamp TIMESTAMPTZ NOT NULL,
    trader TEXT NOT NULL,
    symbol TEXT NOT NULL,
    raw_data JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for tracking specific whales over time
-- "Follow the scent trail of a particular whale"
CREATE INDEX idx_scent_tracker ON whale_scents(trader, timestamp DESC);

-- Index for finding events by block height
-- "Search the blockchain timeline"
CREATE INDEX idx_scent_trail ON whale_scents(block_height);

-- Index for filtering by event type
-- "Categorize the types of scents"
CREATE INDEX idx_scent_type ON whale_scents(event_type);

-- Index for finding the freshest scents
-- "What's happening right now?"
CREATE INDEX idx_fresh_scents ON whale_scents(timestamp DESC);

-- GIN index for searching within the raw JSON data
-- "Deep dive into the scent details"
CREATE INDEX idx_scent_data ON whale_scents USING GIN (raw_data);

-- Index for symbol-based queries
-- "Which whales are trading this asset?"
CREATE INDEX idx_scent_symbol ON whale_scents(symbol, timestamp DESC);

-- Table comments for documentation
COMMENT ON TABLE whale_scents IS 'Raw blockchain events tracked by Tracehound - the digital scent trail of whale activity';
COMMENT ON COLUMN whale_scents.id IS 'Unique identifier for each scent record';
COMMENT ON COLUMN whale_scents.tx_hash IS 'Blockchain transaction hash - the whale''s pawprint';
COMMENT ON COLUMN whale_scents.block_height IS 'Block number where this scent was detected';
COMMENT ON COLUMN whale_scents.event_type IS 'Type of whale activity: trade, position, or liquidation';
COMMENT ON COLUMN whale_scents.timestamp IS 'When this activity occurred on the blockchain';
COMMENT ON COLUMN whale_scents.trader IS 'Whale account address - who we''re tracking';
COMMENT ON COLUMN whale_scents.symbol IS 'Trading pair or asset symbol (e.g., ETH, BTC)';
COMMENT ON COLUMN whale_scents.raw_data IS 'Complete event data in JSON format';
COMMENT ON COLUMN whale_scents.created_at IS 'When Tracehound recorded this scent';

-- Create a view for recent whale activity (last 24 hours)
CREATE OR REPLACE VIEW recent_whale_activity AS
SELECT
    trader,
    symbol,
    event_type,
    COUNT(*) as event_count,
    MAX(timestamp) as last_seen
FROM whale_scents
WHERE timestamp > NOW() - INTERVAL '24 hours'
GROUP BY trader, symbol, event_type
ORDER BY event_count DESC;

COMMENT ON VIEW recent_whale_activity IS 'Summary of whale activity in the last 24 hours';

-- Create a view for top whales by activity
CREATE OR REPLACE VIEW top_whales AS
SELECT
    trader,
    COUNT(*) as total_events,
    COUNT(DISTINCT symbol) as symbols_traded,
    MIN(timestamp) as first_seen,
    MAX(timestamp) as last_seen
FROM whale_scents
GROUP BY trader
ORDER BY total_events DESC
LIMIT 100;

COMMENT ON VIEW top_whales IS 'Top 100 most active whales tracked by Tracehound';
