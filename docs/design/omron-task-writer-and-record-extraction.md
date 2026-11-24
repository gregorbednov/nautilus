# Per-task writer и независимое извлечение task-record

18 ноября 2026 года. Продолжение [таблицы task-файлов](omron-task-file-table.md).
Найден per-task вызов compiler file writer и разобран query перед передачей.
В portable Go добавлено точное извлечение task-record для сравнения и анализа.
Оно не подменяет standalone task-file или квалифицированный transfer payload.

## Где записывается task-файл

Worker124b7fa0 строит имя по ранее найденному правилу:
baseName.Left(length-3) + hex task number. Вызов124b8f47 идёт через1285ab02
в именованный PLC_KBase::CompileMultipleProgramsAndProduceBinaryFile121c28ef.

В stack sequence124b8f14..124b8f47 первым отложен указатель созданного имени,
затем флаг, затем результат1280edf0 в позиции task selector.1280edf0 читает
поле descriptor+4. Это getter scalar, а не дополнительный файловый writer.
При входе в compiler writer явный путь занимает `[EBP+20]`.
Последующие writer calls и N1-сериализация прослежены статически; вызовов
нативного компилятора в этом этапе не выполнялось.

Writer разделяет whole-image и selected-task N1 ветви:

| Listing | Подтверждённое условие |
| --- | --- |
|121c5760/121c5767 | header1320 bytes пишется только при selectorffff |
|121c5cb9/121c5cc0 | directory1152 bytes пишется только при selectorffff |
|121c5ffe/121c6004 | task header пишется при совпадении descriptor+204 с selector, либоffff |
|121c6042/121c6048 | аналогичный gate для wrapper/body/trailer |

В обычной N1 task ветви видны32-word header, нулевое слово, compiler wrapper,
условный cyclic IR/DR prologue, code helper121d6647 и trailerbb800f48.
Full N1 parser уже определяет эти границы по длине записи и проверяет wrapper.

Но writer также имеет отдельные условные блоки для других типов программ,
включая ветвь descriptor flags&20 с23-word header. Их включение зависит от
selector и внутренних program IDs. Поэтому **нельзя объявлять произвольный
slice task-record полным результатом per-task компиляции**, особенно при FB.
Ветви N2/ULC и protected preparation также не обобщаются на plain CJ1 N1.

## Новое независимое API

```go
record, err := um.ExtractN1TaskRecord(fullN1, taskNumber)
```

API сначала валидирует весь N1 через ParseN1, находит запись и возвращает
собственную копию её bytes. Границы —OffsetWords/LengthWords из проверенного
каталога. Код не перекодирует инструкции, не меняет headers, code-page bytes,
неизвестные слова или IR/DR wrapper и не добавляет FB dependencies.

Неверный N1 сохраняет ErrN1Layout. Отсутствующая задача возвращает
ErrN1TaskNotFound, пригодный дляerrors.Is. Сеть и vendor DLL не используются.
API пока не подключён к CLI и не добавляет task write client.

Проверены оба retained native full-image образца, CJ1H и CJ1M, отсутствие
aliasing в обе стороны, непрозрачные header/name/code bytes, отсутствующие
задачи, некорректный N1 и synthetic cyclic IR/DR wrapper. В transfer test
извлечённый record retained no-FB fixture отвергается full-program planner;
оригинальный N1 сохраняет прежние qualification flags=false.
Из-за отсутствия identifying magic у N1 это проверка конкретных поддержанных
fixtures, а не универсальный детектор всех возможных форматов.

Вне Git сохранены hashes4 записей из двух full images. Для CJ1H:
task256 =752 bytes, task257 =704 bytes. Это **record slices**, не native
standalone task golden. Ограниченный поиск numeric task suffix files вprojects,
research/cj1-oracle и верхнем Temp/*.100/*.101 кандидатов не дал; глобальное
отсутствие таких файлов в системе не заявляется.

## Query1263f9a0 перед task download

Этот helper не пишет содержимое task-файла. Он формирует внутренний запрос
с read selector0306 либо0316, выбирает адрес и запрашивает4 bytes.
Callback1263fda0 принимает сообщение ожидаемого вида и вызывает1263fde0.
Decoder проверяет внутренний тип1fd, длину/маркер4 и собирает значение из
bytes+18..+1b в big endian. Это смещения **внутреннего CDM message**, не
доказанные смещения FINS wire response.

Три profile-mask getters читают descriptor+18. Для flags&1c возвращаются
8000/4000/0c00. Другая ветвь сbit20 даёт02000000/01000000/00300000.
Возвращаемые boolean outputs формируются через AND с прочитанным значением.
Имена конкретных трёх outputs не назначены только по положению аргументов:
сигнатуры pseudo-C местами неверны, а выбор реального runtime profile не измерен.

При protected ветви12699b50 вызывает12850070 — ранее исследованный helper
CompositeUMObject. Он не исполнялся и не переносился в backend.
Наличие read selector само по себе не квалифицирует все вложенные calls,
полный workflow доступа/режимов или их применимость к действующей установке.
Новый сетевой status-query в Go не добавлен.

## Доказательства и проверки

Git: `omron/um/testdata/task-file-writer-evidence.json.gz`,10 полных functions,
5 report hashes, provenance CX-P.exe / CX-Pdata.dll.
Compressed SHA-256:
`bf40c7b2e125e0630ccf6609f00fbfbcf9e4c02cb19e6a002884945314e6e6c5`.
Исходные binary hashes закреплены в evidence и Go native-evidence test.

Offline verifier: **516/516**, включая PE source/report hashes, E8 calls,
MFC/CX-Pdata import identities, writer export и record-slice arithmetic replay.
Прошли `go test ./omron/um ./omron/transfer` и
`go vet ./omron/um ./omron/transfer`.
Raw: `research/hardware/20261118-cj1-task-writer-01`.
Manifest: 23 файлов, SHA-256 `7d0b5fa66420b613705573b7fb9d131a99f00b5f6d906e162a7d55070dbe776b`.

Следом — native standalone task golden и точные правила auxiliary/FB blocks,
затем отдельная спецификация task payload. До этого извлечённые records
используются для анализа и сравнения, не для отправки на CPU.
Аппаратная финализация/восстановление остаются отдельными этапами.
ПЛК не опрашивались; .37 READ ONLY, .36 offline; vendor-код не выполнялся.

Продолжение: [capture task selection](omron-capture-task-selection.md) сохраняет
селекторы и пути в аудите и уточняет, почему Ctrl+F7 пока не является
подтверждённым источником standalone task golden.
