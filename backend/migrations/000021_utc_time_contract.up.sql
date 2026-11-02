CREATE TABLE time_contract (
 id tinyint NOT NULL PRIMARY KEY,
 contract_version int NOT NULL,
 origin varchar(32) NOT NULL,
 offset_seconds int NOT NULL,
 operation_id varchar(64) NOT NULL,
 manifest_sha256 char(64) NOT NULL,
 completed_at datetime(6) NOT NULL,
 CHECK (id=1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
