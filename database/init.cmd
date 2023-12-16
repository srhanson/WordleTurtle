CREATE TABLE `results` (
    `wordlenum` INTEGER,
    `userId` VARCHAR(64),
    `displayName` VARCHAR(64),
    `score` INTEGER,
    `hardmode` INTEGER,
    `timestamp` DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (wordlenum, userId)
);