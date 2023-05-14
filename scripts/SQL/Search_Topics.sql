--// Normal topics //--
SELECT * FROM Topics AS t WHERE MATCH(Name) AGAINST('Something' IN NATURAL LANGUAGE MODE);

--// Archived topics //--
SELECT * FROM TopicsArchived AS t WHERE MATCH(Name) AGAINST('Something' IN NATURAL LANGUAGE MODE);

--// All topics //--
SELECT * FROM Topics AS t WHERE MATCH(Name) AGAINST('Something' IN NATURAL LANGUAGE MODE)
UNION
SELECT * FROM TopicsArchived AS t WHERE MATCH(Name) AGAINST('Something' IN NATURAL LANGUAGE MODE);
