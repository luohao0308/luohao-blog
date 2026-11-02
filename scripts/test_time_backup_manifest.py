import gzip
import importlib.util
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('backup_manifest', ROOT / 'deploy/time_backup_manifest.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


def dump(extra=b'', version=20):
    return (b'CREATE TABLE `schema_migrations` (\n  `version` bigint NOT NULL,\n  `dirty` tinyint NOT NULL\n) ENGINE=InnoDB;\n'
            + f'INSERT INTO `schema_migrations` (`version`, `dirty`) VALUES ({version},0);\n'.encode()
            + b'CREATE TABLE `empty_table` (\n `id` int\n) ENGINE=InnoDB;\n'
            + b'CREATE TABLE `content` (\n `id` int,\n `body` text\n) ENGINE=InnoDB;\n'
            + extra + b'/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;\n')


class ManifestTests(unittest.TestCase):
    def inspect(self, content):
        with tempfile.TemporaryDirectory() as directory:
            p = Path(directory) / 'dump.gz'
            p.write_bytes(gzip.compress(content))
            return m.inspect_dump(p)

    def test_quotes_json_null_and_empty_tables(self):
        row = b'''INSERT INTO `content` (`id`, `body`) VALUES (1,'{"date":"2026-11-02T14:50:32Z","body":"a\\nb\\'c"}');\n'''
        result = self.inspect(dump(row))
        self.assertEqual(result['source_schema_version'], 20)
        self.assertEqual([x['rows'] for x in result['tables']], [1, 0, 1])
        self.inspect(dump(b'INSERT INTO `content` (`id`, `body`) VALUES (1,NULL);\n'))

    def test_truncation_dirty_and_extended_insert_refused(self):
        for data in [dump()[:-20], dump(version=22), dump(b'INSERT INTO `content` (`id`, `body`) VALUES (1,NULL),(2,NULL);\n'), dump(b'INSERT INTO `missing` (`id`) VALUES (1);\n')]:
            with self.subTest(data=data[-50:]), self.assertRaises(ValueError):
                self.inspect(data)

    def test_redundant_charset_is_canonical_but_real_changes_are_not(self):
        before = [b'CREATE TABLE `test` (\n', b'  `body` varchar(32) COLLATE utf8mb4_bin NOT NULL,\n', b') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;\n']
        after = [before[0], b'  `body` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,\n', before[2]]
        self.assertEqual(m.canonical_ddl(before), m.canonical_ddl(after))
        wrong_collation = [before[0], after[1].replace(b'utf8mb4_bin', b'utf8mb4_general_ci'), before[2]]
        wrong_charset = [before[0], after[1].replace(b'CHARACTER SET utf8mb4', b'CHARACTER SET latin1'), before[2]]
        self.assertNotEqual(m.canonical_ddl(before), m.canonical_ddl(wrong_collation))
        self.assertNotEqual(m.canonical_ddl(before), m.canonical_ddl(wrong_charset))

    def test_literal_text_is_not_normalized_and_comparison_rejects_changes(self):
        literal = b"  `body` varchar(32) DEFAULT 'COLLATE utf8mb4_bin CHARACTER SET utf8mb4' COMMENT 'COLLATE utf8mb4_bin'\n"
        tail = b') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;\n'
        lines = [b'CREATE TABLE `test` (\n', literal, tail]
        self.assertEqual(m.canonical_ddl(lines), b''.join(lines))
        actual = literal.replace(b'varchar(32) DEFAULT', b'varchar(32) COLLATE utf8mb4_bin DEFAULT')
        expected = actual.replace(b'varchar(32) COLLATE', b'varchar(32) CHARACTER SET utf8mb4 COLLATE', 1)
        self.assertEqual(m.canonical_ddl([lines[0], actual, tail]), lines[0] + expected + tail)
        base = dump().replace(b' `body` text', b'  `body` varchar(32) COLLATE utf8mb4_bin DEFAULT NULL').replace(b'ENGINE=InnoDB;', b'ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;')
        normalized = base.replace(b'varchar(32) COLLATE', b'varchar(32) CHARACTER SET utf8mb4 COLLATE')
        m.compare(self.inspect(base), self.inspect(normalized))
        for changed in [base.replace(b'varchar(32) COLLATE utf8mb4_bin', b'varchar(32) COLLATE utf8mb4_general_ci'), base.replace(b'varchar(32) COLLATE utf8mb4_bin', b'varchar(32) CHARACTER SET latin1 COLLATE latin1_bin')]:
            with self.assertRaises(ValueError):
                m.compare(self.inspect(base), self.inspect(changed))

    def test_compare_rejects_extra_marker_and_changed_rows(self):
        old = self.inspect(dump())
        with self.assertRaises(ValueError):
            self.inspect(dump(b'CREATE TABLE `time_contract` (\n `id` int\n) ENGINE=InnoDB;\n'))
        extra = self.inspect(dump(b'CREATE TABLE `time_contract` (\n `id` int\n) ENGINE=InnoDB;\nINSERT INTO `time_contract` (`id`) VALUES (1);\n', version=21))
        with self.assertRaises(ValueError):
            m.compare(old, extra)
        changed = self.inspect(dump(b'INSERT INTO `content` (`id`, `body`) VALUES (1,NULL);\n'))
        with self.assertRaises(ValueError):
            m.compare(old, changed)
        m.compare(old, old)


if __name__ == '__main__':
    unittest.main()
