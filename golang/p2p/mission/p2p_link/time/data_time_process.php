<?php

header('access-control-allow-credentials: true');
header('Access-Control-Allow-Headers: *');
header('access-control-allow-methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS');
header('Access-Control-Allow-Origin: *');
header('server: hust.media');
header('Content-type: application/json; charset=UTF-8');
header('x-hustmedia-region: AWS - ap-southeast-1');
include '../../../config_main_2.php';

$sourceFile = __DIR__ . '/data_time_get_full.json';
$outputFile = __DIR__ . '/data_time_process.json';

if (!file_exists($sourceFile)) {
    echo json_encode([], JSON_PRETTY_PRINT);
    exit;
}

$rawData = file_get_contents($sourceFile);
$missions = json_decode($rawData, true);

if (!is_array($missions)) {
    echo json_encode([], JSON_PRETTY_PRINT);
    exit;
}

$grouped = [];
foreach ($missions as $mission) {
    $category = $mission['api_category'] ?? 'unknown';
    if (!isset($grouped[$category])) {
        $grouped[$category] = [
            'api_category' => $category,
            'total_mission' => 0,
            'data_mission' => []
        ];
    }
    $grouped[$category]['total_mission']++;
    $cleanMission = $mission;
    unset($cleanMission['api_category']);
    $updated = isset($mission['mission_updatedate']) ? strtotime($mission['mission_updatedate']) : null;
    $created = isset($mission['mission_createdate']) ? strtotime($mission['mission_createdate']) : null;
    $cleanMission['mission_second'] = ($updated && $created) ? max(0, $updated - $created) : null;
    $grouped[$category]['data_mission'][] = $cleanMission;
}

$result = array_values($grouped);
$json = json_encode($result, JSON_PRETTY_PRINT);

file_put_contents($outputFile, $json);
echo $json;
