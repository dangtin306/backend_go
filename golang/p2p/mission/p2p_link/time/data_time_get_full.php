<?php

header('access-control-allow-credentials: true');
header('Access-Control-Allow-Headers: *');
header('access-control-allow-methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS');
header('Access-Control-Allow-Origin: *');
header('server: hust.media');
header('Content-type: application/json; charset=UTF-8');
header('x-hustmedia-region: AWS - ap-southeast-1');
include '../../../config_main_2.php';

$missions = mysqli_query(
    $play_sql,
    "SELECT `id`, `api_category`, `iduser`, `status`, `category_code`, `phone`, `mission_updatedate`
    FROM `misson_shorten`
    WHERE `status` = 'Completed'
    ORDER BY `id` DESC"
);

$output = [];
while ($mission = mysqli_fetch_assoc($missions)) {
    $missionId = $mission['id'];
    $linksResult = mysqli_query(
        $play_sql,
        "SELECT `mission_createdate`
        FROM `misson_shorten_link`
        WHERE `id_misson` = '" . $missionId . "'
        ORDER BY `id` DESC"
    );
    $latestLink = mysqli_fetch_assoc($linksResult);
    if ($latestLink) {
        $mission['mission_createdate'] = $latestLink['mission_createdate'];
    }
    $output[] = $mission;
}

$json = json_encode($output, JSON_PRETTY_PRINT);
file_put_contents(__DIR__ . '/data_time_get_full.json', $json);
echo $json;
