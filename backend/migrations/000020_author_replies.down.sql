-- Forward-only: preserve author replies and their moderation evidence.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'author replies require a forward compatible release';
