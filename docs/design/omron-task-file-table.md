# Таблица task-файлов перед передачей

18 ноября 2026 года. Продолжение [разделения task и full-program](omron-task-versus-program-dispatch.md).
Прослежено126519b0→44-byte записи→dispatcher12610780→task helper12611fd0.
Таблица содержит отдельные имена файлов задач и метаданные; она не является
новым заголовком N1 и не служит wire format.

## Состояние рабочей копии

При начале этого этапа фактический HEAD былdfae2637. Коммита24f7fa8a и
последних raw-снимков, обсуждавшихся в предыдущих сообщениях, в данной копии
нет; объект24f7fa8a отсутствует, reflog также заканчиваетсяdfae2637.
Старого репозитория E:\cx-proger\nautilus нет. Восстановление тех этапов
не заявляется. Это исследование выполнено заново по доступным pinned binaries
и сохранено поверх фактической базы, без reset/переключения веток.

## Builder и запись в памяти

12610210 передаёт копию CString this+78 в126519b0.
Builder очищает массив через12652390, обходит модель проекта через126496f0 /
12649760, формирует записи и сохраняет имена обратно в объекты модели.

12652390 — управление динамическим массивом. Адрес массива находится
в owner+d0, число записей — owner+d4. Расчёт allocation/index:
`(n+2*(5*n))*4 = 44*n`. Внутренний array object передаётся как owner+cc;
его buffer/count/capacity/grow находятся по+4/+8/+c/+10.
Эти смещения нельзя смешивать со смещениями самого owner.

Подтверждённые поля44-byte записи:

| Смещение | Наблюдаемое использование |
| --- | --- |
|00 | scalar; ненулевые записи пропускаются исследованной task-ветвью |
|04 | CString имени файла, передаваемый task helper |
|08 | CString, используемый в диагностике126130a0 |
|0c | числовой аргумент задачи для12611fd0 |
|20 | ещё один CString |
|1c,24 | однобайтные поля |

Полный смысл прочих полей пока не установлен.
Copy helper12652530 присваивает CString по+4/+8/+20 через1285b132;
это не memcpy сериализованной структуры.32-bit указатели и MFC ownership
не должны попадать в переносимый backend как формат данных.

## Временные и производные имена

1285a988 — PE thunk в именованный CX-Pdata::MakeTemporaryFilePath12177d40.
Builder вызывает его с префиксами **SEC** и **PRG**. Реализация helper
обращается к GetTempPathA/GetTempFileNameA; последний получает unique=0.
Поэтому генератор нельзя считать чистой операцией над строками или безопасно
исполнять как офлайн oracle без проверки побочных эффектов. Мы его не исполняли.

Если в соответствующей ветви входное имя непустое, builder выполняет:

```text
CString::Format("%03X", numeric task argument)
CString::Left(baseName, byteLength(baseName)-3)
operator+(leftPart, formattedNumber)
```

Например, для обычного ASCII путиproject.obj и значения0x100 результат
арифметического построения имени —project.100. Это минимальная ширина3,
не правило обрезания hex-числа до3 символов. Применимость конкретных номеров
и имя реального файла не доказываются синтетическим примером.

12648980 находит объект по числовому полю+34 и присваивает CString по+3c.
12648a10 обходит вложенные объекты и присваивает CString по+44.
1264bf90/1264bb70 копируют метаданные и вложенные объекты. В этих helpers
есть виртуальные вызовы; полный рекурсивный аудит их побочных эффектов не закрыт.

## Связь с task download

В12610780 индекс owner+e0 сравнивается с count+d4; затем вычисляется
`record = buffer+d0 + index*44`. При ненулевом record[0] исследованная ветвь
переходит дальше. Для записи с нулевым полем:

```text
12610bb3 MOV EDX,[record+0c]
12610bb6 MOV EAX,[record+04]
12610bb9 PUSH EDX
12610bba PUSH EAX
12610bbd CALL12611fd0
```

12611fd0 использует имя для CFile::GetStatus и формирует запрос task-сервиса
463/CDM_DOWNLOAD_TASK либо526/CDM_DOWNLOAD_TASK_EX, как установлено раньше.
Это отдельная ветвь от полной загрузки: standard CDMDownloadProgram12610ab3
получает имя непосредственно из owner+78, а не из этой записи.
Существующий full-program0307 planner не получает task semantics автоматически.

## Проверка непосредственно перед task download

126130a0, вызываемый из12611fd0, тоже не установлен как writer.
Он обходит ту же таблицу по44 bytes и обращается к именованным KBase methods
GetTaskProtectFlag (1285abfe) и GetTaskOperationStartFlag (1285abf8),
формируя предупреждения и условия продолжения. Вызов1263f9a0 внутри него
ещё требует отдельного разбора. Это не разрешение менять защиту или режим CPU.
Значения флагов с действующего контроллера здесь не читались.

## Проверки и границы

В Git: `omron/transfer/testdata/task-table-evidence.json.gz`,17 полных
listings/pseudo-C, hashes7 raw reports, provenance двух pinned binaries.
Compressed SHA-256:
`d7cb3d054d7f4588703278b0ef9a249e6c2d6f45f186c8c2491b0bd9dc0ab232`.
Source hashes CX-P / CX-Pdata:
`e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4`,
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.

Go native-evidence test закрепляет размер и CString-поля записи, построение
имён, обновление модели, consumer аргументы и разделение с полной загрузкой.
Он работает без Windows/CX-One/сети. Прошли:
`go test ./omron/transfer ./omron/um`, `go vet ./omron/transfer`.

Offline verifier: **435/435**, включая прямые E8 calls против PE bytes,
MFC/CX-Pdata imports, temporary helper export/API, literal bytes и ограниченный
арифметический replay индексации. Replay не исполняет native-код.
Raw: `research/hardware/20261118-cj1-task-table-01`.
Manifest: 34 файлов, SHA-256 `5c4b113bba50c93c5e94689541a55db337cb0aa47b414d6bb55a604f273dd733`.

Остаются: место записи содержимого task-файлов,1263f9a0 и виртуальные callees,
реальное соответствие task payload/raw N1 и полный путь bytes до wire.
Нового transfer API не добавлено. WireImageMappingQualified/WriteQualified=false.
ПЛК не опрашивались; .37 READ ONLY, .36 offline. Native golden и аппаратная
верификация по-прежнему не подменяются статическим исследованием.

Продолжение: [per-task writer и извлечение record](omron-task-writer-and-record-extraction.md)
находит запись через compiler worker и разбирает1263f9a0. Standalone task file
по-прежнему требует отдельного native golden и проверки auxiliary blocks.
