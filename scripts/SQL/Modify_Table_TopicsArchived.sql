--// This script adds indices to the `TopicsArchived` table //--
CREATE FULLTEXT INDEX TopicsArchived_Name_FTIDX ON TopicsArchived (Name);
CREATE INDEX TopicsArchived_Name_BTIDX USING BTREE ON TopicsArchived (Name);
