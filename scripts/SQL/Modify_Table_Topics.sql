--// This script adds indices to the `Topics` table //--
CREATE FULLTEXT INDEX Topics_Name_FTIDX ON Topics (Name);
CREATE INDEX Topics_Name_BTIDX USING BTREE ON Topics (Name);
