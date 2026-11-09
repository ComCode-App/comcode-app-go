<?php
// Temporary owner-operated export tool. PHP 7.4+, PDO MySQL and zlib; no Composer.
const EXPORT_TOKEN = 'CHANGE_ME_TO_A_LONG_RANDOM_TOKEN';
const EXPORT_PUBLIC_ROOT = ''; // Absolute public_html path; default: parent of this random export folder.
const EXPORT_CACHE_PATH = ''; // Optional absolute path OUTSIDE public_html.
const LEGACY_CONFIG_PATH = ''; // Absolute path to a PHP config or Laravel .env, outside web root.
// Non-empty explicit values override discovered values. Empty password is allowed.
const EXPORT_DB_HOST = '';
const EXPORT_DB_PORT = '';
const EXPORT_DB_NAME = '';
const EXPORT_DB_USER = '';
const EXPORT_DB_PASS = '';

ini_set('display_errors', '0');
ini_set('log_errors', '0'); // PDO exceptions can contain credentials; never log them here.
header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
header('Pragma: no-cache');
header('Expires: 0');
header('X-Robots-Tag: noindex, nofollow, noarchive');
header('Referrer-Policy: no-referrer');
header('X-Content-Type-Options: nosniff');
if (EXPORT_TOKEN === 'CHANGE_ME_TO_A_LONG_RANDOM_TOKEN' || strlen(EXPORT_TOKEN) < 32 ||
    !isset($_GET['token']) || !is_string($_GET['token']) || !hash_equals(EXPORT_TOKEN, $_GET['token'])) {
    http_response_code(404);
    header('Content-Type: text/plain; charset=UTF-8');
    exit('Not Found');
}

// Parse literal assignments only: never include/eval a legacy config with application side effects.
function configValues($path) {
    if ($path === '') return array();
    $text = @file_get_contents($path);
    if ($text === false) throw new RuntimeException('config');
    $values = array();
    if (strtolower(pathinfo($path, PATHINFO_EXTENSION)) !== 'php') {
        foreach (preg_split('/\r\n|\n|\r/', $text) as $line) {
            if (preg_match('/^\s*(?:export\s+)?(DB_[A-Z_]+)\s*=\s*(.*?)\s*$/', $line, $m)) {
                $v = $m[2];
                if (strlen($v) >= 2 && ($v[0] === '"' || $v[0] === "'") && substr($v, -1) === $v[0]) {
                    $v = substr($v, 1, -1);
                } else {
                    $v = preg_replace('/\s+#.*$/', '', $v);
                }
                $values[$m[1]] = $v;
            }
        }
        return $values;
    }
    // token_get_all discards commented-out credentials and avoids interpreting PHP expressions.
    $tokens = token_get_all($text);
    $literal = function ($token) {
        if (!is_array($token) || $token[0] !== T_CONSTANT_ENCAPSED_STRING) return null;
        $v = substr($token[1], 1, -1);
        if ($token[1][0] === "'") return str_replace(array("\\'", "\\\\"), array("'", "\\"), $v);
        return stripcslashes($v);
    };
    $t = array();
    foreach ($tokens as $token) {
        if (is_array($token) && in_array($token[0], array(T_WHITESPACE, T_COMMENT, T_DOC_COMMENT), true)) continue;
        $t[] = $token;
    }
    for ($i = 0; $i < count($t); $i++) {
        $name = null;
        $value = null;
        if (is_array($t[$i]) && $t[$i][0] === T_STRING && strtolower($t[$i][1]) === 'define' &&
            ($t[$i+1] ?? null) === '(' && ($t[$i+3] ?? null) === ',' && ($t[$i+5] ?? null) === ')') {
            $name = $literal($t[$i+2]);
            $value = $literal($t[$i+4]);
        } elseif (is_array($t[$i]) && in_array($t[$i][0], array(T_VARIABLE, T_CONST), true)) {
            $n = $i;
            if ($t[$i][0] === T_CONST) $n++;
            if (is_array($t[$n] ?? null) && ($t[$n+1] ?? null) === '=' && ($t[$n+3] ?? null) === ';') {
                $name = ltrim($t[$n][1], '$');
                $value = $literal($t[$n+2]);
            }
        }
        if ($name !== null && $value !== null) $values[strtoupper($name)] = $value;
    }
    return $values;
}

function identifier($name) { return '`' . str_replace('`', '``', $name) . '`'; }
function sampleValue($v) {
    if ($v === null) return null;
    $v = (string)$v;
    if (function_exists('mb_substr')) return mb_substr($v, 0, 80, 'UTF-8');
    if (preg_match('/^.{0,80}/us', $v, $m)) return $m[0];
    return substr($v, 0, 80);
}


function allowedTables() { return array('users', 'hits', 'main_link_types', 'pages'); }
function safeColumns($table, $columns) {
    $secret = array('password','lost_password_code','email_activation_code','facebook_id','instagram_id','ip');
    // The sync never uses visitor or signup IP addresses, so they never leave the host.
    return array_values(array_filter($columns, function ($c) use ($table, $secret) {
        if ($table === 'hits') return strtolower($c) !== 'ip';
        return $table !== 'users' || !in_array(strtolower($c), $secret, true);
    }));
}
function mediaRoots() { return array('uploads/avatars','uploads/comps','uploads/kataloglar','usercard','template/images'); }
function mediaType($path) {
    $types = array('jpg'=>'image/jpeg','jpeg'=>'image/jpeg','png'=>'image/png','gif'=>'image/gif',
        'webp'=>'image/webp','avif'=>'image/avif','ico'=>'image/x-icon','svg'=>'image/svg+xml','pdf'=>'application/pdf');
    return $types[strtolower(pathinfo($path, PATHINFO_EXTENSION))] ?? null;
}
function publicRoot() {
    $root = realpath(EXPORT_PUBLIC_ROOT !== '' ? EXPORT_PUBLIC_ROOT : dirname(__DIR__));
    if ($root === false) throw new RuntimeException('root');
    return $root;
}
function mediaPath($root, $rel) {
    if (!is_string($rel) || strpos($rel, "\0") !== false || strpos($rel, '\\') !== false ||
        substr($rel,0,1) === '/' || in_array('..', explode('/', $rel), true) || !mediaType($rel)) return false;
    $allowed = false;
    foreach (mediaRoots() as $r) if (strpos($rel, $r . '/') === 0) $allowed = true;
    if (!$allowed) return false;
    $part = $root;
    foreach (explode('/', $rel) as $segment) {
        $part .= '/' . $segment;
        if (is_link($part)) return false;
    }
    $real = realpath($part);
    if ($real === false || strpos($real, $root . '/') !== 0 || !is_file($real)) return false;
    return $real;
}
function mediaFiles($root, $roots) {
    foreach ($roots as $r) {
        if (!in_array($r, mediaRoots(), true)) throw new RuntimeException('roots');
        $dir = $root . '/' . $r;
        if (!is_dir($dir) || is_link($dir)) continue;
        $it = new RecursiveIteratorIterator(new RecursiveDirectoryIterator($dir, FilesystemIterator::SKIP_DOTS));
        foreach ($it as $f) {
            $rel = substr($f->getPathname(), strlen($root)+1);
            if (mediaPath($root, $rel) !== false) yield $rel => $f;
        }
    }
}
function tarHeader($path, $size, $mtime) {
    $name = $path; $prefix = '';
    if (strlen($name) > 100) {
        $pos = strrpos(substr($path, 0, 156), '/');
        if ($pos === false) throw new RuntimeException('tar path');
        $prefix = substr($path, 0, $pos); $name = substr($path, $pos+1);
    }
    if (strlen($name)>100 || strlen($prefix)>155) throw new RuntimeException('tar path');
    $h = str_pad($name,100,"\0") . sprintf('%07o',0644)."\0" . str_repeat("0000000\0",2) .
        sprintf('%011o',$size)."\0" . sprintf('%011o',$mtime)."\0" . str_repeat(' ',8) . '0' .
        str_repeat("\0",100) . "ustar\0" . '00' . str_repeat("\0",80) . str_pad($prefix,155,"\0") . str_repeat("\0",12);
    $sum = array_sum(unpack('C*',$h));
    return substr_replace($h, sprintf('%06o',$sum)."\0 ",148,8);
}
function exportMedia($mode) {
    @set_time_limit(0);
    ini_set('zlib.output_compression', '0');
    while (ob_get_level() > 0) ob_end_clean();
    $root = publicRoot();
    if ($mode === 'media_file') {
        $path = mediaPath($root, $_GET['path'] ?? '');
        if ($path === false) { http_response_code(404); exit('Not Found'); }
        header('Content-Type: '.mediaType($path)); header('Content-Length: '.filesize($path));
        if (readfile($path) === false) throw new RuntimeException('read');
        return;
    }
    $roots = mediaRoots();
    if (isset($_GET['roots'])) {
        if (!is_string($_GET['roots'])) throw new RuntimeException('roots');
        $roots = array_unique(explode(',', $_GET['roots']));
        foreach ($roots as $r) if (!in_array($r, mediaRoots(), true)) throw new RuntimeException('roots');
    }
    if ($mode === 'media_manifest') {
        $cachePath = EXPORT_CACHE_PATH !== '' ? EXPORT_CACHE_PATH : dirname($root).'/.comcode-export-cache.json';
        $parent = realpath(dirname($cachePath));
        $cacheOK = $parent !== false && $parent !== $root && strpos($parent, $root.'/') !== 0 && !is_link($cachePath);
        $cache = $cacheOK ? json_decode(@file_get_contents($cachePath) ?: '{}',true) : array();
        if (!is_array($cache)) $cache = array();
        $next = array();
        header('Content-Type: application/json; charset=UTF-8'); echo '['; $sep='';
        foreach (mediaFiles($root,$roots) as $rel=>$f) {
            $key = $rel.'|'.$f->getSize().'|'.$f->getMTime();
            $sha = $cache[$key] ?? sha1_file($f->getPathname());
            if ($sha === false) throw new RuntimeException('hash');
            $next[$key]=$sha;
            echo $sep.json_encode(array('path'=>$rel,'size'=>$f->getSize(),'mtime'=>$f->getMTime(),'sha1'=>$sha),JSON_THROW_ON_ERROR);
            $sep=',';
        }
        echo ']';
        if ($cacheOK) {
            $tmp = @tempnam($parent,'.comcode-cache-');
            if ($tmp !== false) {
                @chmod($tmp,0600);
                if (@file_put_contents($tmp,json_encode($next)) !== false) @rename($tmp,$cachePath);
                if (is_file($tmp)) @unlink($tmp);
            }
        }
        return;
    }
    $since = $_GET['since'] ?? '0';
    if (!is_string($since) || !ctype_digit($since)) throw new RuntimeException('since');
    header('Content-Type: application/gzip'); header('Content-Disposition: attachment; filename="legacy-media.tar.gz"');
    $gzip=deflate_init(ZLIB_ENCODING_GZIP);
    if ($gzip === false) throw new RuntimeException('gzip');
    foreach (mediaFiles($root,$roots) as $rel=>$f) {
        if ($f->getMTime() <= (int)$since) continue;
        echo deflate_add($gzip,tarHeader($rel,$f->getSize(),$f->getMTime()),ZLIB_NO_FLUSH);
        $in=fopen($f->getPathname(),'rb');
        if ($in === false) throw new RuntimeException('read');
        $remaining=$f->getSize();
        while ($remaining>0) {
            $data=fread($in,min(65536,$remaining));
            if ($data === false || $data === '') throw new RuntimeException('read');
            $remaining-=strlen($data); echo deflate_add($gzip,$data,ZLIB_NO_FLUSH);
        }
        fclose($in);
        $padding=(512-$f->getSize()%512)%512;
        if ($padding) echo deflate_add($gzip,str_repeat("\0",$padding),ZLIB_NO_FLUSH);
        flush();
    }
    echo deflate_add($gzip,str_repeat("\0",1024),ZLIB_FINISH);
}

$streaming = false;
try {
    $mode = $_GET['mode'] ?? 'schema';
    if (!is_string($mode) || !in_array($mode, array('schema', 'dump', 'hits_since', 'media_manifest', 'media_file', 'media_bundle'), true)) throw new RuntimeException('mode');
    if (strpos($mode, 'media_') === 0) { exportMedia($mode); exit; }
    if ($mode === 'dump' && !in_array($_GET['table'] ?? '', allowedTables(), true)) { http_response_code(404); exit('Not Found'); }
    $cfg = configValues(LEGACY_CONFIG_PATH);
    $host = EXPORT_DB_HOST !== '' ? EXPORT_DB_HOST : ($cfg['DB_HOST'] ?? 'localhost');
    $port = EXPORT_DB_PORT !== '' ? EXPORT_DB_PORT : ($cfg['DB_PORT'] ?? '3306');
    $name = EXPORT_DB_NAME !== '' ? EXPORT_DB_NAME : ($cfg['DB_NAME'] ?? $cfg['DB_DATABASE'] ?? '');
    $user = EXPORT_DB_USER !== '' ? EXPORT_DB_USER : ($cfg['DB_USER'] ?? $cfg['DB_USERNAME'] ?? '');
    $pass = EXPORT_DB_PASS !== '' ? EXPORT_DB_PASS : ($cfg['DB_PASS'] ?? $cfg['DB_PASSWORD'] ?? '');
    if ($name === '' || $user === '' || !ctype_digit((string)$port) ||
        strpbrk($host . $name, ";\r\n\0") !== false) throw new RuntimeException('credentials');
    $pdo = new PDO('mysql:host=' . $host . ';port=' . $port . ';dbname=' . $name . ';charset=utf8mb4', $user, $pass, array(
        PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
        PDO::ATTR_EMULATE_PREPARES => false,
        PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        PDO::MYSQL_ATTR_USE_BUFFERED_QUERY => false,
    ));
    $q = $pdo->query('SHOW FULL TABLES');
    $tables = array();
    while ($r = $q->fetch(PDO::FETCH_NUM)) {
        if ($r[1] === 'BASE TABLE' && in_array($r[0], allowedTables(), true)) $tables[] = $r[0];
    }
    $q->closeCursor();
    sort($tables);
    if ($mode === 'schema') {
        $result = array('reminder' => 'Cutover tamamlanınca bu dosyayı FTP üzerinden SİLİN. Delete after cutover.', 'tables' => array());
        foreach ($tables as $table) {
            $quoted = identifier($table);
            $q = $pdo->query('SHOW COLUMNS FROM ' . $quoted);
            $columns = array();
            while ($r = $q->fetch()) { if (count(safeColumns($table,array($r['Field']))) === 0) continue; $columns[] = array('name' => $r['Field'], 'type' => $r['Type'], 'nullable' => $r['Null'] === 'YES', 'key' => $r['Key']); }
            $q->closeCursor();
            $q = $pdo->query('SELECT COUNT(*) FROM ' . $quoted);
            $count = $q->fetchColumn();
            $q->closeCursor();
            $safe = safeColumns($table, array_column($columns,'name'));
            $q = $pdo->query('SELECT ' . implode(',',array_map('identifier',$safe)) . ' FROM ' . $quoted . ' LIMIT 3');
            $sample = array();
            while ($r = $q->fetch()) $sample[] = array_map('sampleValue', $r);
            $q->closeCursor();
            $result['tables'][] = array('name' => $table, 'columns' => $columns, 'row_count' => (int)$count, 'sample' => $sample);
        }
        header('Content-Type: application/json; charset=UTF-8');
        echo json_encode($result, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_INVALID_UTF8_SUBSTITUTE | JSON_THROW_ON_ERROR);
        exit;
    }
    $table = $mode === 'hits_since' ? 'hits' : ($_GET['table'] ?? '');
    if (!is_string($table) || !in_array($table, $tables, true)) {
        http_response_code(404);
        header('Content-Type: text/plain; charset=UTF-8');
        exit('Not Found');
    }
    if (!function_exists('deflate_init')) throw new RuntimeException('zlib');
    $cols = $pdo->query('SHOW COLUMNS FROM '.identifier($table));
    $names = array(); while ($c=$cols->fetch()) $names[]=$c['Field']; $cols->closeCursor();
    $select = 'SELECT '.implode(',',array_map('identifier',safeColumns($table,$names))).' FROM '.identifier($table);
    if ($mode === 'hits_since') {
        $since=$_GET['since'] ?? '';
        if (!is_string($since) || !preg_match('/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/D',$since)) throw new RuntimeException('since');
        $q=$pdo->prepare($select.' WHERE `date` > ? ORDER BY `date`,`id`'); $q->execute(array($since));
    } else { $q=$pdo->query($select); }
    $header = array();
    for ($i = 0; $i < $q->columnCount(); $i++) $header[] = $q->getColumnMeta($i)['name'];
    $gzip = deflate_init(ZLIB_ENCODING_GZIP);
    $rowBuffer = fopen('php://memory', 'w+'); // One CSV row only, never a server file.
    if ($gzip === false || $rowBuffer === false) throw new RuntimeException('stream');
    @set_time_limit(0);
    ini_set('zlib.output_compression', '0');
    while (ob_get_level() > 0) ob_end_clean();
    header('Content-Type: application/gzip');
    $filename = preg_replace('/[^a-zA-Z0-9_.-]/', '_', $table);
    header('Content-Disposition: attachment; filename="' . $filename . '.csv.gz"');
    header('X-Accel-Buffering: no');
    $streaming = true;
    $writeRow = function ($row) use ($rowBuffer, $gzip) {
        rewind($rowBuffer);
        ftruncate($rowBuffer, 0);
        // Disable PHP's proprietary escape mechanism; RFC 4180 quoting round-trips backslashes.
        if (fputcsv($rowBuffer, $row, ',', '"', '') === false) throw new RuntimeException('csv');
        rewind($rowBuffer);
        while (!feof($rowBuffer)) {
            $chunk = fread($rowBuffer, 65536);
            if ($chunk === false) throw new RuntimeException('read');
            $out = deflate_add($gzip, $chunk, ZLIB_NO_FLUSH);
            if ($out === false) throw new RuntimeException('gzip');
            echo $out;
        }
    };
    $writeRow($header);
    $n = 0;
    while ($row = $q->fetch(PDO::FETCH_NUM)) {
        $writeRow($row);
        if (++$n % 128 === 0) {
            echo deflate_add($gzip, '', ZLIB_SYNC_FLUSH);
            flush();
            if (connection_aborted()) exit;
        }
    }
    echo deflate_add($gzip, '', ZLIB_FINISH);
    fclose($rowBuffer);
    $q->closeCursor();
} catch (Throwable $e) {
    // A truncated gzip is intentionally invalid: never disguise a failed export as a complete file.
    if (!$streaming) {
        http_response_code(500);
        header('Content-Type: text/plain; charset=UTF-8');
        echo 'Export failed. Check configuration and server requirements.';
    }
}
