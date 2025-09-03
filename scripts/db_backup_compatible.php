<?php
/**
 * Universal Database Backup Script (PHP 5.3+ Compatible)
 * Supports MySQL, PostgreSQL, SQLite
 * 
 * Usage:
 * GET: ?type=mysql&host=localhost&port=3306&user=root&pass=password&db=database_name
 * GET: ?type=postgresql&host=localhost&port=5432&user=postgres&pass=password&db=database_name
 * GET: ?type=sqlite&path=/path/to/database.sqlite
 * 
 * Security: Add authentication token if needed
 * Compatible with PHP 5.3+
 */

// Security headers
header('Content-Type: application/json');
header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: GET, POST');
header('Access-Control-Allow-Headers: Content-Type');

// Error reporting
error_reporting(E_ALL);
ini_set('display_errors', 0);

// Function to send JSON response
function sendResponse($success, $message, $data = null) {
    $response = array(
        'success' => $success,
        'message' => $message,
        'timestamp' => date('Y-m-d H:i:s')
    );
    
    if ($data !== null) {
        $response['data'] = $data;
    }
    
    // Use JSON_PRETTY_PRINT if available (PHP 5.4+), otherwise compact format
    if (defined('JSON_PRETTY_PRINT')) {
        echo json_encode($response, JSON_PRETTY_PRINT);
    } else {
        echo json_encode($response);
    }
    exit;
}

// Function to validate input
function validateInput($type, $params) {
    $required = array();
    
    switch ($type) {
        case 'mysql':
        case 'postgresql':
            $required = array('host', 'port', 'user', 'pass', 'db');
            break;
        case 'sqlite':
            $required = array('path');
            break;
        default:
            return false;
    }
    
    foreach ($required as $field) {
        if (!isset($params[$field]) || empty($params[$field])) {
            return false;
        }
    }
    
    return true;
}

// Function to backup MySQL database
function backupMySQL($params) {
    $host = $params['host'];
    $port = $params['port'];
    $user = $params['user'];
    $pass = $params['pass'];
    $db = $params['db'];
    
    try {
        // Create PDO connection
        $dsn = "mysql:host=$host;port=$port;dbname=$db;charset=utf8mb4";
        $pdo = new PDO($dsn, $user, $pass, array(
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        ));
        
        // Get all tables
        $stmt = $pdo->query("SHOW TABLES");
        $tables = $stmt->fetchAll(PDO::FETCH_COLUMN);
        
        $dump = "-- MySQL Database Dump\n";
        $dump .= "-- Generated: " . date('Y-m-d H:i:s') . "\n";
        $dump .= "-- Database: $db\n\n";
        $dump .= "SET FOREIGN_KEY_CHECKS=0;\n\n";
        
        foreach ($tables as $table) {
            // Get table structure
            $stmt = $pdo->query("SHOW CREATE TABLE `$table`");
            $createTable = $stmt->fetch();
            $dump .= $createTable['Create Table'] . ";\n\n";
            
            // Get table data
            $stmt = $pdo->query("SELECT * FROM `$table`");
            $rows = $stmt->fetchAll();
            
            if (!empty($rows)) {
                $dump .= "INSERT INTO `$table` VALUES\n";
                $values = array();
                
                foreach ($rows as $row) {
                    $rowValues = array();
                    foreach ($row as $value) {
                        if ($value === null) {
                            $rowValues[] = 'NULL';
                        } else {
                            $rowValues[] = "'" . addslashes($value) . "'";
                        }
                    }
                    $values[] = "(" . implode(',', $rowValues) . ")";
                }
                
                $dump .= implode(",\n", $values) . ";\n\n";
            }
        }
        
        $dump .= "SET FOREIGN_KEY_CHECKS=1;\n";
        
        return array(
            'dump' => $dump,
            'tables_count' => count($tables),
            'format' => 'sql'
        );
        
    } catch (Exception $e) {
        throw new Exception("MySQL backup failed: " . $e->getMessage());
    }
}

// Function to backup PostgreSQL database
function backupPostgreSQL($params) {
    $host = $params['host'];
    $port = $params['port'];
    $user = $params['user'];
    $pass = $params['pass'];
    $db = $params['db'];
    
    try {
        // Create PDO connection
        $dsn = "pgsql:host=$host;port=$port;dbname=$db";
        $pdo = new PDO($dsn, $user, $pass, array(
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        ));
        
        // Get all tables
        $stmt = $pdo->query("SELECT tablename FROM pg_tables WHERE schemaname = 'public'");
        $tables = $stmt->fetchAll(PDO::FETCH_COLUMN);
        
        $dump = "-- PostgreSQL Database Dump\n";
        $dump .= "-- Generated: " . date('Y-m-d H:i:s') . "\n";
        $dump .= "-- Database: $db\n\n";
        
        foreach ($tables as $table) {
            // Get table structure
            $stmt = $pdo->query("SELECT column_name, data_type, is_nullable, column_default 
                                FROM information_schema.columns 
                                WHERE table_name = '$table' AND table_schema = 'public' 
                                ORDER BY ordinal_position");
            $columns = $stmt->fetchAll();
            
            $dump .= "CREATE TABLE $table (\n";
            $columnDefs = array();
            
            foreach ($columns as $column) {
                $def = $column['column_name'] . ' ' . $column['data_type'];
                if ($column['is_nullable'] === 'NO') {
                    $def .= ' NOT NULL';
                }
                if ($column['column_default']) {
                    $def .= ' DEFAULT ' . $column['column_default'];
                }
                $columnDefs[] = $def;
            }
            
            $dump .= implode(",\n", $columnDefs) . "\n);\n\n";
            
            // Get table data
            $stmt = $pdo->query("SELECT * FROM $table");
            $rows = $stmt->fetchAll();
            
            if (!empty($rows)) {
                $dump .= "INSERT INTO $table VALUES\n";
                $values = array();
                
                foreach ($rows as $row) {
                    $rowValues = array();
                    foreach ($row as $value) {
                        if ($value === null) {
                            $rowValues[] = 'NULL';
                        } else {
                            $rowValues[] = "'" . addslashes($value) . "'";
                        }
                    }
                    $values[] = "(" . implode(',', $rowValues) . ")";
                }
                
                $dump .= implode(",\n", $values) . ";\n\n";
            }
        }
        
        return array(
            'dump' => $dump,
            'tables_count' => count($tables),
            'format' => 'sql'
        );
        
    } catch (Exception $e) {
        throw new Exception("PostgreSQL backup failed: " . $e->getMessage());
    }
}

// Function to backup SQLite database
function backupSQLite($params) {
    $path = $params['path'];
    
    if (!file_exists($path)) {
        throw new Exception("SQLite database file not found: $path");
    }
    
    try {
        $pdo = new PDO("sqlite:$path", null, null, array(
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        ));
        
        // Get all tables
        $stmt = $pdo->query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'");
        $tables = $stmt->fetchAll(PDO::FETCH_COLUMN);
        
        $dump = "-- SQLite Database Dump\n";
        $dump .= "-- Generated: " . date('Y-m-d H:i:s') . "\n";
        $dump .= "-- Database: $path\n\n";
        
        foreach ($tables as $table) {
            // Get table structure
            $stmt = $pdo->query("SELECT sql FROM sqlite_master WHERE type='table' AND name='$table'");
            $createTable = $stmt->fetch();
            $dump .= $createTable['sql'] . ";\n\n";
            
            // Get table data
            $stmt = $pdo->query("SELECT * FROM $table");
            $rows = $stmt->fetchAll();
            
            if (!empty($rows)) {
                $dump .= "INSERT INTO $table VALUES\n";
                $values = array();
                
                foreach ($rows as $row) {
                    $rowValues = array();
                    foreach ($row as $value) {
                        if ($value === null) {
                            $rowValues[] = 'NULL';
                        } else {
                            $rowValues[] = "'" . addslashes($value) . "'";
                        }
                    }
                    $values[] = "(" . implode(',', $rowValues) . ")";
                }
                
                $dump .= implode(",\n", $values) . ";\n\n";
            }
        }
        
        return array(
            'dump' => $dump,
            'tables_count' => count($tables),
            'format' => 'sql'
        );
        
    } catch (Exception $e) {
        throw new Exception("SQLite backup failed: " . $e->getMessage());
    }
}

// Main execution
try {
    // Get request method
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $params = $_GET;
    } elseif ($method === 'POST') {
        $params = $_POST;
    } else {
        sendResponse(false, 'Unsupported request method');
    }
    
    // Validate required parameters
    if (!isset($params['type'])) {
        sendResponse(false, 'Database type is required (mysql, postgresql, sqlite)');
    }
    
    $type = strtolower(trim($params['type']));
    
    if (!validateInput($type, $params)) {
        sendResponse(false, 'Invalid or missing parameters for database type: ' . $type);
    }
    
    // Perform backup based on database type
    $result = null;
    
    switch ($type) {
        case 'mysql':
            $result = backupMySQL($params);
            break;
        case 'postgresql':
            $result = backupPostgreSQL($params);
            break;
        case 'sqlite':
            $result = backupSQLite($params);
            break;
        default:
            sendResponse(false, 'Unsupported database type: ' . $type);
    }
    
    // Return successful result
    sendResponse(true, 'Database backup completed successfully', $result);
    
} catch (Exception $e) {
    sendResponse(false, $e->getMessage());
}
?>
