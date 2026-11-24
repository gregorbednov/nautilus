# CJ1: manufacturing CPU read на обоих ПЛК — 2026-11-19

После [статической трассировки selector/consumer](omron-cj1-manufacturing-consumer.md)
выполнено ограниченное read-only чтение0226 selector0/index0 на CJ1H и CJ1M.
Первый этап использовал существующий `Client.Call`, второй — новый
`Client.ReadCPUManufacturingData`. Оба этапа записали полный обмен TCP.

| ПЛК | Подтверждённая0501 модель | DATA bytes | DATA21..23 | Отображение CX-One по native-коду | Revision |
| --- | --- | --- | --- | --- | --- |
| 10.0.174.37:9600 | CJ1H_CPU65H | 26 | 03 00 09 | 3.0 | 9 |
| 10.0.174.36:9600 | CJ1M_CPU21 | 26 | 03 00 06 | 3.0 | 6 |

Это фактически прочитанные bytes и результат подтверждённого статического
форматирования `%X.%X` / `%X`. Позже19 ноября пользователь прислал два
скриншота PLC Information: в Actual Characteristics обоих CPU указана
Unit Ver.3.0. Независимое подтверждение Unit Version получено.
Профили компилятора/передачи автоматически не изменялись.

## Подтверждение из CX-Programmer

| Actual Type | Unit Ver. | Revision | PCB Revision | Software Revision | Lot Number |
| --- | --- | --- | --- | --- | --- |
| CJ1M CPU21 | 3.0 | I | AAA | BG 6 | 050120 |
| CJ1H-H CPU65 | 3.0 | Q | FB- | BJ 9 | 060331 |

Показания Actual Characteristics и Manufacturing Details прочитаны с
присланных изображений. Revision в этой таблице — буквенное поле окна,
а не DATA23. Суффиксы Software Revision6/9 согласуются с DATA23;
отдельное поле Unit Version Revision Number на этих скриншотах не показано.
Lot, буквенная Revision, PCB и префиксы BG/BJ также совпали с сохранённым DATA.
IP-адреса в окнах не видны; связь с ранее исследованными целями опирается
на контекст пользователя, типы CPU и совпавшие manufacturing данные.

Оба окна показывают `Protected: No`. Это фиксируется как отдельное UI-наблюдение:
его эквивалентность внутреннему IsExtendedProtect или выбору0307/0317
ещё не доказана. Версию3.0 нельзя заменять прежним library default4.0.

Скриншоты и подтверждение сохранены отдельным пакетом
`C:/cx-proger/research/hardware/20261119-cj1-manufacturing-ui-01`.
12 files excluding SHA256SUMS; manifest SHA256:
`62ee8208f2a68600835bb844770f222a0411555420417035614109d44058a260`.
Исторические read-capture JSON и тестовая hardware fixture содержат
`unitVersionIndependentlyVerified=false` на момент чтения и не переписывались.
В последующем `confirmation.json` зафиксировано true для Unit Version;
WriteQualified/RestoreQualified остаются false.

В каждом этапе сняты две копии с каждого CPU. Все четыре копии каждого CPU
совпали, включая совпадение generic и typed API. DATA SHA256:

- CJ1H: `5def45b53199bcf414df2515527b6496b4a7fa36c7e542d3951d5fffd721e128`;
- CJ1M: `c4e2bbb067e29b199dd8e087194800ee82a6f8b896603ebd51706329750e04d3`.

Native guard объекта версии прошёл на обоих: YYMM bytes0603 и0501.
Это не отдельная календарная проверка. Первые bytes ответа — часть
manufacturing данных, а не echo selector: наблюдались51 и49, при запросе0.
Хвост DATA24..25 сохранён без трактовки.

## Обмен и границы опыта

По одному свежему TCP-соединению на CPU в каждом из двух этапов.
Последовательность после node negotiation:
`0501 → 0601 → 0226 → 0226 → 0501 → 0601`.
Оба запроса0226 имеют ровно четыре нулевых bytes параметров.
Ни retries после ошибок, ни перебора selector не было.

Сырые журналы независимо разобраны `verify_capture.py`, включая TCP framing,
handshake, destination node37/36, route, SID, command, нулевой end code,
модель0501, длину, совпадение сохранённых DATA и before/after JSON.
Всего в двух этапах:8 запросов0501,8 запросов0601,8 запросов0226.
Команд управления, записи UM или параметров нет.
На каждом соединении до и после чтения: RUN(mode4), fatal0/nonfatal0,
CPU/status Raw совпадают. Память программы этим опытом не перечитывалась;
он не квалифицирует полный backup/restore или запись программы.

## Go API

`omron/fins/manufacturing.go` добавляет:

- `ReadCPUManufacturingData(ctx)` — один фиксированный0226/0000/0000,
  без повторов; caller отдельно устанавливает модель CPU;
- `DecodeCPUManufacturingData(DATA)` — офлайн-разбор без vendor-кода и сети;
- `CPUManufacturingData` — собственная копия Raw, исходные bytes версии/YYMM,
  native display и отдельный результат guard объекта версии.

Ответ проверяется под общей блокировкой клиента после проверки
route/SID/command/end code. Требуются26..1000 bytes DATA и соблюдение меньшего
предела транспорта.1000 — защитный предел бэкенда, не native maximum.
Дополнительные bytes сохраняются. Короткий/слишком длинный payload закрывает
сессию; любой ненулевой end code возвращается без маскирования status bits.
Если транспорт не вмещает26 bytes, запроса нет.

Диалоговое отображение не зависит от native date guard: это две наблюдаемые
ветви CX-One. Guard не проверяет календарь и не должен подменяться проверкой
реальной даты. Hex bytes12/0a отображаются12.A, не18.10. Blank triplet
отображается `-`; zero triplet сохраняется как0.0/0 без утверждения реальной
версии CPU. Производственный бэкенд остаётся независимым от Windows/CX-One.

## Проверки

Прошли `go test ./omron/...` для всех10 пакетов и `go vet ./omron/...`.
Все41 Python-тест исследовательских инструментов также прошли.
Go-тесты используют сохранённые ответы обоих реальных CPU и проверяют
фиксированный запрос, независимое владение Raw/полями, native display/date
boundaries, blank/zero triplet, extensions, все короткие DATA0..25,
ограничения транспорта, отказ от повреждённой сессии и ненулевые end codes.

Новый typed API затем проверен на обоих ПЛК, а его наблюдения сверены с
независимым Python inspector и первым аппаратным набором. Одни и те же
исходные ответы проходят сохранённый статический и независимый wire-разбор.
Host Link аппаратно в этом опыте не проверялся.

Frozen evidence:
`C:/cx-proger/research/hardware/20261119-cj1-manufacturing-read-01`.
59 files excluding SHA256SUMS; manifest SHA256:
`a68d20318e5ecaf56ef835517c3d1f356e0ef8829928099cbcd4d26d1ab3bb9b`.
Переносимая hardware fixture SHA256:
`df6a5f360a5b530557801e374cd27a8292179fd1715cc68850182f75041f52ae`.
Предыдущие frozen archives не изменялись.

Переносимая фикстура: `omron/fins/testdata/cj1-manufacturing-observed.json`.
История профилей с library default/current4.0 остаётся отдельно:
совпадение/различие чисел не позволяет автоматически выбрать сервис,
IsExtendedProtect или параметры финализации.

Далее: сопоставить подтверждённую Unit Version3.0 с условиями сервисов
и ограничениями независимого профиля. Дополнительное подтверждение этой
версии от оператора больше не требуется.
Исследование IsExtendedProtect, выбор0307/0317, финализация и опыт
полной записи/возврата исходной программы остаются отдельными gates.
`WriteQualified=false`, `RestoreQualified=false`.
