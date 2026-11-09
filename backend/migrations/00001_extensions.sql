-- +goose Up
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS postgis;

-- +goose Down
-- Extensions are shared infrastructure; rolling back app tables must not drop
-- TimescaleDB/PostGIS or any future spatial/time-series data.
SELECT 1;
