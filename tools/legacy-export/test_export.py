"""Local PHP behavior checks without a legacy MySQL server (Python stdlib only)."""
import gzip
import io
import tarfile
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SOURCE = Path(__file__).with_name("export.php").read_text()
PHP = shutil.which("php")
ROOT_TMP = Path(__file__).resolve().parents[2] / "tmp"


@unittest.skipUnless(PHP, "PHP is not installed")
class ExportTest(unittest.TestCase):
    def setUp(self):
        ROOT_TMP.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT_TMP)
        self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name)

    def php(self, code, *args):
        result = subprocess.run([PHP, "-r", code, *map(str, args)], capture_output=True, check=True)
        self.assertEqual(result.stderr, b"")
        return result.stdout.decode()

    def auth(self, token_constant, supplied):
        source = SOURCE.replace("'CHANGE_ME_TO_A_LONG_RANDOM_TOKEN';", json.dumps(token_constant) + ";", 1)
        path = self.folder / "export.php"
        path.write_text(source)
        output = self.php('$_GET=json_decode($argv[1],true); include $argv[2];', json.dumps(supplied), path)
        return output

    def test_auth_fails_closed(self):
        for configured, supplied in [
            ("CHANGE_ME_TO_A_LONG_RANDOM_TOKEN", {"token": "CHANGE_ME_TO_A_LONG_RANDOM_TOKEN"}),
            ("short", {"token": "short"}),
            ("t" * 64, {}),
            ("t" * 64, {"token": "wrong"}),
            ("t" * 64, {"token": ["t" * 64]}),
        ]:
            with self.subTest(configured=configured, supplied=supplied):
                self.assertEqual(self.auth(configured, supplied), "Not Found")

    def test_valid_auth_never_exposes_config_error(self):
        self.assertEqual(self.auth("t" * 64, {"token": "t" * 64}),
                         "Export failed. Check configuration and server requirements.")

    def config(self, filename, content):
        path = self.folder / filename
        path.write_text(content)
        # Exercise the actual shipped parser, without executing its HTTP/DB entry point.
        helpers = SOURCE[SOURCE.index("function configValues"):SOURCE.index("$streaming = false;")]
        output = self.php(helpers + '\necho json_encode(configValues($argv[1]));', path)
        return json.loads(output)

    def test_php_literal_config_and_comments(self):
        cfg = self.config("config.php", """<?php
// define('DB_PASS', 'comment-secret');
define('DB_HOST', 'localhost');
const DB_NAME = 'old_cards';
$db_user = 'cpanel_reader';
$DB_PASS = 'quote\\\'and\\\\slash';
file_put_contents('/should-never-run', 'x');
""")
        self.assertEqual(cfg, {"DB_HOST": "localhost", "DB_NAME": "old_cards",
                               "DB_USER": "cpanel_reader", "DB_PASS": "quote'and\\slash"})

    def test_php_expressions_not_evaluated(self):
        cfg = self.config("config.php", "<?php $DB_PASS = getenv('PASSWORD'); define('DB_HOST', 'local'.'host');")
        self.assertEqual(cfg, [])

    def test_env_config(self):
        cfg = self.config(".env", '# ignored\nDB_DATABASE=old_cards\nDB_USERNAME="reader"\nDB_PASSWORD=\'spaces # intact\'\nDB_HOST=localhost # comment\n')
        self.assertEqual(cfg, {"DB_DATABASE": "old_cards", "DB_USERNAME": "reader",
                              "DB_PASSWORD": "spaces # intact", "DB_HOST": "localhost"})

    def test_sample_and_identifier(self):
        helpers = SOURCE[SOURCE.index("function configValues"):SOURCE.index("$streaming = false;")]
        output = json.loads(self.php(helpers + "\necho json_encode(array(sampleValue(str_repeat('ğ',100)),sampleValue(null),identifier('a`b')));"))
        self.assertEqual(output, ["ğ" * 80, None, "`a``b`"])

    def test_settings_refused_before_database(self):
        self.assertEqual(self.auth("t" * 64, {"token": "t" * 64, "mode": "dump", "table": "settings"}), "Not Found")

    def test_sensitive_column_exclusion(self):
        helpers = SOURCE[SOURCE.index("function configValues"):SOURCE.index("$streaming = false;")]
        value = self.php(helpers + "echo json_encode(safeColumns('users', array('user_id','password','email','lost_password_code','email_activation_code','facebook_id','instagram_id','ip')));")
        self.assertEqual(json.loads(value), ["user_id", "email"])
        value = self.php(helpers + "echo json_encode(safeColumns('hits', array('id','type','type_id','user_id','date','ip')));")
        self.assertEqual(json.loads(value), ["id", "type", "type_id", "user_id", "date"])

    def media(self, mode, **params):
        root = self.folder / "public_html"
        source = SOURCE.replace("'CHANGE_ME_TO_A_LONG_RANDOM_TOKEN';", json.dumps("t"*64)+";", 1)
        source = source.replace("const EXPORT_PUBLIC_ROOT = '';", "const EXPORT_PUBLIC_ROOT = " + json.dumps(str(root)) + ";")
        path = self.folder / "export.php"
        path.write_text(source)
        result = subprocess.run([PHP, "-r", '$_GET=json_decode($argv[1],true); include $argv[2];', json.dumps(dict(token="t"*64, mode=mode, **params)), str(path)], capture_output=True, check=True)
        self.assertEqual(result.stderr, b"")
        return result.stdout

    def test_media_modes_and_paths(self):
        root = self.folder / "public_html"
        folder = root / "uploads" / "avatars"
        folder.mkdir(parents=True)
        (folder / "a.png").write_bytes(b"image bytes")
        (folder / "secret.php").write_bytes(b"never export")
        (folder / "linked.png").symlink_to(folder / "a.png")
        manifest = json.loads(self.media("media_manifest"))
        self.assertEqual([x["path"] for x in manifest], ["uploads/avatars/a.png"])
        self.assertEqual(len(manifest[0]["sha1"]), 40)
        self.assertEqual(self.media("media_file", path="uploads/avatars/a.png"), b"image bytes")
        for path in ["uploads/avatars/../../secret.png", "uploads/avatars/linked.png", "uploads/avatars/secret.php", "/etc/passwd"]:
            self.assertEqual(self.media("media_file", path=path), b"Not Found")
        bundle = self.media("media_bundle", since="0", roots="uploads/avatars")
        with tarfile.open(fileobj=io.BytesIO(bundle), mode="r:gz") as archive:
            self.assertEqual(archive.getnames(), ["uploads/avatars/a.png"])
            self.assertEqual(archive.extractfile(archive.getnames()[0]).read(), b"image bytes")
        with tarfile.open(fileobj=io.BytesIO(self.media("media_bundle", since="9999999999")), mode="r:gz") as archive:
            self.assertEqual(archive.getnames(), [])

    def fixture_export(self, params):
        stub = r'''
class ExportFixtureStatement {
    public $rows; public $names; public $index=0; public $filter=false;
    function __construct($rows,$names=array()) { $this->rows=$rows; $this->names=$names; }
    function fetch($mode=null) {
        if ($this->index>=count($this->rows)) return false;
        $row=$this->rows[$this->index++];
        return $mode===PDO::FETCH_NUM ? array_values($row) : $row;
    }
    function closeCursor() {}
    function fetchColumn() { return count($this->rows); }
    function columnCount() { return count($this->names); }
    function getColumnMeta($i) { return array('name'=>$this->names[$i]); }
    function execute($params) {
        if ($this->filter) $this->rows=array_values(array_filter($this->rows,function($r)use($params){return $r['date']>$params[0];}));
    }
}
class ExportFixturePDO {
    function __construct(...$args) {}
    function prepare($sql) { $s=$this->query($sql); $s->filter=true; return $s; }
    function query($sql) {
        if ($sql==='SHOW FULL TABLES') return new ExportFixtureStatement(array(array('users','BASE TABLE'),array('hits','BASE TABLE'),array('settings','BASE TABLE')));
        $users=strpos($sql,'`users`')!==false;
        $names=$users?array('user_id','email','password','lost_password_code','email_activation_code','facebook_id','instagram_id'):array('id','type','user_id','date');
        if (strpos($sql,'SHOW COLUMNS')===0) return new ExportFixtureStatement(array_map(function($n){return array('Field'=>$n,'Type'=>'text','Null'=>'YES','Key'=>'');},$names));
        if ($users) {
            foreach (array('password','lost_password_code','email_activation_code','facebook_id','instagram_id') as $s) if(strpos($sql,'`'.$s.'`')!==false) throw new RuntimeException('secret selected');
            return new ExportFixtureStatement(array(array('user_id'=>'1','email'=>'owner@example.test')),array('user_id','email'));
        }
        return new ExportFixtureStatement(array(array('id'=>'1','type'=>'profile','user_id'=>'1','date'=>'2020-01-01 00:00:00'),array('id'=>'2','type'=>'profile','user_id'=>'1','date'=>'2026-10-19 01:00:00')),array('id','type','user_id','date'));
    }
}
'''
        source = SOURCE.replace("<?php", "<?php\n" + stub, 1).replace("new PDO(", "new ExportFixturePDO(")
        source = source.replace("'CHANGE_ME_TO_A_LONG_RANDOM_TOKEN';", json.dumps("t"*64)+";", 1)
        source = source.replace("const EXPORT_DB_NAME = '';", "const EXPORT_DB_NAME = 'fixture';")
        source = source.replace("const EXPORT_DB_USER = '';", "const EXPORT_DB_USER = 'reader';")
        path = self.folder / "fixture.php"
        path.write_text(source)
        result = subprocess.run([PHP, "-r", '$_GET=json_decode($argv[1],true); include $argv[2];', json.dumps(dict(token="t"*64, **params)), str(path)], capture_output=True, check=True)
        self.assertEqual(result.stderr, b"")
        return result.stdout

    def test_dump_excludes_secrets(self):
        import csv
        data = list(csv.reader(io.StringIO(gzip.decompress(self.fixture_export(dict(mode="dump", table="users"))).decode())))
        self.assertEqual(data, [["user_id", "email"], ["1", "owner@example.test"]])

    def test_hits_since_streams_only_newer_rows(self):
        import csv
        data = list(csv.reader(io.StringIO(gzip.decompress(self.fixture_export(dict(mode="hits_since", since="2026-10-19 00:00:00"))).decode())))
        self.assertEqual(data[0], ["id", "type", "user_id", "date"])
        self.assertEqual([row[0] for row in data[1:]], ["2"])


if __name__ == "__main__":
    unittest.main()
