DROP INDEX IF EXISTS `ix_updated` ON `gym`;
DROP INDEX IF EXISTS `ix_old_forts` ON `gym`;
ALTER TABLE `gym` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `gym` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `gym` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;
ALTER TABLE `gym` ADD INDEX `ix_updated` (`updated`), ADD INDEX `ix_old_forts` (`cell_id`, `deleted`, `updated`);

ALTER TABLE `incident` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `incident` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `incident` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;

DROP INDEX IF EXISTS `ix_updated` ON `pokemon`;
ALTER TABLE `pokemon` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NULL DEFAULT NULL;
UPDATE `pokemon` SET `updated_ms` = COALESCE(`updated_ms`, 0) * 1000;
ALTER TABLE `pokemon` MODIFY COLUMN `updated_ms` BIGINT UNSIGNED NOT NULL DEFAULT 0;
ALTER TABLE `pokemon` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;

DROP INDEX IF EXISTS `ix_updated` ON `pokestop`;
DROP INDEX IF EXISTS `ix_old_forts` ON `pokestop`;
ALTER TABLE `pokestop` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `pokestop` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `pokestop` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;
ALTER TABLE `pokestop` ADD INDEX `ix_updated` (`updated`), ADD INDEX `ix_old_forts` (`cell_id`, `deleted`, `updated`);

DROP INDEX IF EXISTS `ix_updated` ON `s2cell`;
ALTER TABLE `s2cell` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `s2cell` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `s2cell` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;
ALTER TABLE `s2cell` ADD INDEX `ix_updated` (`updated`);

DROP INDEX IF EXISTS `ix_updated` ON `spawnpoint`;
ALTER TABLE `spawnpoint` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL DEFAULT 0;
UPDATE `spawnpoint` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `spawnpoint` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;
ALTER TABLE `spawnpoint` ADD INDEX `ix_updated` (`updated`);

DROP INDEX IF EXISTS `ix_updated` ON `station`;
ALTER TABLE `station` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `station` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `station` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;
ALTER TABLE `station` ADD INDEX `ix_updated` (`updated`);

ALTER TABLE `station_battle` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `station_battle` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `station_battle` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;

ALTER TABLE `weather` CHANGE COLUMN `updated` `updated_ms` BIGINT UNSIGNED NOT NULL;
UPDATE `weather` SET `updated_ms` = `updated_ms` * 1000;
ALTER TABLE `weather` ADD COLUMN `updated` INT UNSIGNED GENERATED ALWAYS AS (`updated_ms` DIV 1000) VIRTUAL AFTER `updated_ms`;
