# Передача разработки Omron на Windows-стенд

20 ноября. Мастер Nautilus VS Code и команды `omron new-project` и
`omron build-project` создают и импортируют проекты двух CPU. Четыре CXP из
рабочей папки дали образы длиной 4604 байта, каждый совпал с прямой сборкой
того же файла с явными тестовыми настройками. Инвентаризация восьми CXP/OPT
лежит в `research/cj1-project-audit-20261120-01/inventory.json`.
[Проверка обычных инструкций](omron-cj1-ordinary-guards.md) воспроизводит
5888 результатов полной компиляции при Program Check Version 3.0/4.0.

20 November: [full source FB invocation](omron-cj1-source-invocation.md) now
matches public CompileCXPImage and CLI: four 3180-byte complete N1 files at
current/check 3.0/4.0 on both profiles. Source LD must use Initialise/Compile;
the earlier GenerateInst lifecycle skipped input-state processing. Active
stack checks retained: four missing-LD controls rejected with 9 errors. Portable
replay includes 36 guard / 208 stack traces. Earlier negative corpus unchanged.
Next qualify all implemented forms under Program Check Version 3.0, then
protection/finalization and controlled write/original return. No PLC traffic;
explicit codec UnitVersion and runtime/hardware qualification remain closed.

19 November: [native instance binding](omron-cj1-instance-binding.md) now
reconstructs4 complete3092-byte direct-reference images: used definition23words,
instance44words,taskFBICALL, current/check 3.0/4.0. Four generated invocation
frames also match source compiler before full Compile, but full task checking
rejects the invocation recipe with 9 errors. This negative control stays closed:
parameter-stack/source-invocation and hardware qualification remain false.
No PLC traffic. Next resolve native generated-invocation lifecycle.


19 November: [program guard version](omron-cj1-program-guard-version.md) is now
observed separately from KBase current. Previous Compile(...,false) selected
constructor Program Check VersionFF/FF; prior byte equality does not itself
qualify whole-program version checks under3.0. New crossed matrix explicitly
sets program3/4:60 Compile traces and24 independently reconstructed N1 files.
Next referenced FB image must set both versions and retain this trace.
No PLC traffic; production UnitVersion/transfer qualification stays closed.


19 November: [current3 FB/settings stage](omron-cj1-version3-fb.md) retains20
native runs:1532 matching records,100 native rejections,112 Go-reconstructed
N1 images (24 with unreferenced definition). Only single-context BCD FB probes
are enabled for explicit local current3/4. Next compose a referenced FB image
with instance section and task invocation; global explicit UnitVersion3.0,
binary FB and actual transfer/restore remain unqualified. No PLC traffic.


19 November: [current3/current4 instruction and N1 comparison](omron-cj1-version3-compile.md)
completed for 689 BCD/701 binary forms on both models and two no-FB images.
Independent Go words/files match all 5560/16 native results. API/CLI now build
binary N1 without FB; explicit codec UnitVersion and binary FB stay rejected.
Next qualify current3 FB/settings and version-forbidden forms, then resolve
protection/service/finalization before any controlled write/original return.
No PLC traffic or program writes in this stage.


19 November: [manufacturing route](omron-cj1-manufacturing-route.md)
traced with Craft Anything CLI in explicitly permitted Deep mode. Services423/475/481,
0226 candidate constructor and raw extended callback bytes are retained.
No selector guessing, unit-version decoder or PLC traffic was added.

19 November: [local profile-state phase](omron-cj1-local-profile-state.md)
completed after relocation to C:. Both selected library profiles yielded
default/current bytes[4,0,0] and local IsExtendedProtect0. Retained data remain
separate from real target version/protection; transfer qualification is unchanged.

19 November: workspace moved completely to `C:\cx-proger`, repository
`C:\cx-proger\nautilus`, installer `C:\cx`. See
[verified migration](omron-workspace-migration-2026-11-19.md). Historical F:/E:
paths in frozen evidence remain unchanged; use the new root for current work.

19 ноября, после явного разрешения оператора: [mode-only опыт CJ1M](omron-cj1m-mode-only-2026-11-19.md)
успешно завершён. На .36 подтверждены RUN→PROGRAM→RUN, ACK освобождения права,
новый read-only сеанс и побайтная неизменность UM4672 bytes. Этот UM был4672 bytes
уже до управляющих команд (вместо4652 bytes от18 ноября). Записи программы не было.
Первоначальный auto-review отказ и ожидание подтверждения более не блокируют
этот согласованный опыт. Mode-only путь теперь наблюдён на обоих настоящих CPU;
полная загрузка/возврат и hardware qualification остаются открыты.

19 ноября: [controlled session и аппаратный mode-only опыт](omron-cj1-controlled-session.md).
Backup теперь предшествует явным0C01/0402; control intent/ACK/status сохраняются
в журнале, RUN и RELEASE вызываются отдельно. Локальные проверки, включая оба
CPU с файловым журналом, прошли. На настоящем CJ1H/.37 подтверждены
RUN→PROGRAM→RUN и ACK освобождения права; новый read-only сеанс прочитал тот же
UM2716 bytes.0307 не отправлялась, полная загрузка/восстановление не квалифицированы.
Первая попытка CJ1M была отклонена auto-review; затем пользователь явно разрешил
именно этот опыт, и он завершён успешно (см. актуальное обновление выше).

18 ноября, следующий этап: [mode/access API и свежее чтение обоих CPU](omron-cj1-mode-access.md).
Добавлены явные0401/0402/0C01/0C03 с пустым ACK, terminal failure и owner DATA
при3001; localhost-проверки прошли на обоих профилях. RELEASE0C03 глобален,
поэтому автоматического cleanup нет. На настоящих ПЛК только чтение: .37 RUN,
UM2716 bytes с прежним SHA; .36 RUN, UM4652 bytes. Две копии каждого UM и полные
CPU/status до/после совпали; Go backup review прошёл. Дальше — интеграция
control шагов с журналом/соединением ProgramSession; hardware qualification false.

18 ноября, последнее подтверждение оператора: **оба CJ1H/.37 и CJ1M/.36**
снова в сети и отключены от исполнительных механизмов; остались светодиоды
и прочие незначащие нагрузки. Прежний offline/no-poll статус .36 заменён этим
сообщением; доступны чтение и подготовка стендовых проверок обоих CPU.
Это сообщение оператора, не новый результат сетевого/физического измерения.
Mode/access, защита, backup/возврат и аппаратная квалификация остаются открыты.
[Актуальная область доступа](omron-hardware-verification-and-handoff.md).

18 ноября, продолжение: [выделенный program coordinator](omron-cj1-program-session.md)
проверен на localhost: свежая пара UM, синхронный файловый журнал до0307,
полный readback и отдельный однократный возврат приватного исходника через
новую сессию. Старый Client после передачи владения не выполняет I/O/Close.
При ошибке нет retry/resume/автоматического возврата/RUN. Mode/access lifecycle,
защита, full backup и аппаратная приёмка ещё впереди; CLI download отсутствует.
В этом этапе .37/.36 не опрашивались. Изоляция CJ1H подтверждена ранее.

18 ноября: [Go sender одного блока0307](omron-cj1-0307-block-sender.md) реализован
с полным ACK, route/SID correlation и terminal failure даже на PLC end code.
Localhost TCP проверил candidate/readback/original/readback; Host Link528 — на
net.Pipe. ПЛК не затрагивались; CLI download и полный координатор пока отсутствуют.

18 ноября: [свежая пара CJ1H и аудит backup](omron-cj1-program-backup-review.md).
Только чтение .37: два UM по2716 bytes совпали, identity/status стабильны,
режим RUN не менялся. Новый audit-program-backup проверяет до/после и файлы;
это не full backup или квалификация возврата. Найдена ProgChk-ветка записи
version word; она не доказывает физическую unit version. .36 не опрашивался.

18 ноября: оператор подтвердил отключение всех механизмов, способных получить
или нанести ущерб, на CJ1H/.37; подключены только светодиоды. Для этого стенда
отключение исполнительных механизмов подтверждено оператором. Уведомление
не распространяется на CJ1M/.36; его ограничения и offline-статус сохраняются.
Аппаратная стадия ожидает технической готовности загрузки и восстановления;
уведомление само по себе не переводит qualification в true. Подробности —
[действующее ограничение доступа](omron-hardware-verification-and-handoff.md).

18 ноября: [masks и версии контекста](omron-cj1-profile-version-origins.md).
Архивные CJ1M masks=8/0x400/0, CJ1H-H=8/0x80000/0. CurrentVersion может
отличаться от constructor defaults; CS2SupportVersion не переводится в unit version.
Reader полностью не декомпилирован (timeout); закреплён только проверенный фрагмент.

18 ноября: [BR2 и FB-профиль](omron-cj1-br2-and-fb-profile.md): reader/writer/bridge
пропускают BR2; pinned default-кандидат DIFF8191,0. Init проверяет поддержку FB
с DefaultVersion, setter UseFunctionBlockFlag — с CurrentVersion на version-ветке.
Сохранённый флаг не квалифицирует активный профиль; доступ к ПЛК не использовался.

18 ноября: [путь UseDefault](omron-cj1-stack-source-origin.md) прослежен статически.
Missing UseDefault=true пропускает явный stack setter; pinned init-table AC0/64/1536.
При false аргументы: BitSize, WordSize+BitSize. Исправлена безусловная трактовка
сохранённого D0 как конфликта; profile/FB branch и общая квалификация остаются открыты.

18 ноября: [аудит сохранённых настроек](omron-cj1-source-settings-audit.md)
проверяет FBMemory и сохраняет непустой column5 FB-объявлений. Во всех четырёх
CXP повторены D0 против AC0 рецепта и отсутствующий BR[2]. Преобразования и
инициализация не квалифицированы; сеть и vendor runtime не использовались.

18 ноября, продолжение: [локальная пересборка рецепта](omron-cj1-build-reproduction.md)
подключена к preflight через source/settings/resource/timer-mode. Проверяется
весь N1; сохранённый JSON не является доказательством. Остальные ограничения,
включая соответствие настроек проекту/CPU и запрет записи, сохраняются.

18 ноября: добавлена [офлайн-проверка CPU и копий UM](omron-cj1-offline-preflight.md).
0501 декодируется по W342: архивные 10/120 Kwords дают 20480/245760 bytes;
это только граница размера. Сводка сверяется с raw; две программы сравниваются
побайтно. Свежесть, unit version, финализация и полный backup не подтверждены.
Готовность к записи false; в этом этапе обращений к ПЛК не было.

Latest implementation: [offline write audit](omron-cj1-offline-write-audit.md).
ProgramWriteAudit + audit-program-write CLI validate retained blocks/ACKs and
readback. Stopped audits cannot resume; diagnostic inspection leaves failure
intact. Nine synthetic CLI cases exercised; no PLC access or vendor execution.
Next: CPU/version/capacity preflight, route/SID binding, backup/data and transport.

Latest: [UM finalization boundary probes](omron-um-finalization-boundaries.md).
Four readbacks fit both64-byte prefix rules. Portable diagnostics now expose
64/256-byte hypotheses with prefix0/24;2632/2824-byte CXT recipes match native
compiler files for both profiles. The2824-byte probe separates all four rules.
No finalization/writer API or hardware-write authorization was added.
Continue CPU-profile qualification and offline transfer/recovery handling.

Рабочая папка Windows: `F:\cx-proger\nautilus`. Linux-копия:
`/home/greg/cx-proger/nautilus`. Исходная база передачи:
`2db093e1e2e967a3a1b6b09a32fb575a84791748`.

17 ноября рабочая папка перенесена с E: на F:. Старые абсолютные пути внутри
зафиксированных исследовательских материалов сохраняются как provenance;
сами снимки не переписываются. Dev-Environment.ps1 использует PSScriptRoot.
Последний XML-снимок (40 файлов) проверен по manifest, оба Ethernet-профиля
повторно разобраны с F: с точным совпадением результатов. Craft Anything MCP
в этой сессии не подключён; установленная headless Ghidra доступна напрямую.

## Что требуется сделать

Полноценный независимый бэкенд CJ1M/CJ1H: импорт, расширенный LD со всеми
родными инструкциями подходящего CPU, компиляция исполняемой программы,
загрузка/чтение обратно/верификация и мониторинг. Конечные инструменты работают
на Linux без CX-One, Windows, Wine или vendor DLL. Windows — исследовательский
эталон. Ethernet FINS и Host Link обязательны; online edit, Toolbus и полная
эмуляция CJ1 в Go VM отложены.

## Что уже реализовано

18 ноября: oracle audit сохраняетtaskSelector/mode/explicit/default/captured paths,
отклоняет несовпадение известного пути. Full N1 corpus reject observed selector
!=ffff;legacy unknown остаётсянеподтверждённым. Все3 старых captures имеютffff;
2CJ1M complete/path match, старыйCJ1H остаётсяoperator-copy/incomplete.
Resource DLL подтверждаетF7 All PLC иCtrl+F7 Program Compile; primary handler
Ctrl+F7 передаёт12args(3containers+9zero) вworker, normal writer branch→ffff.
Это не готовый standalone task oracle.4 native functions вGit,380/380 checks,
oracle tests/vet/build/CLI verify прошли. [Capture selection](omron-capture-task-selection.md),
raw research/hardware/20261118-cj1-capture-selection-01. Далее local export/
nonzero-selector path. GUI node_repl unavailable,CX-P не запускался;сети не было.

18 ноября, продолжение восстановленной базы: per-task worker124b8f47 вызывает
writer121c28ef с явным путём иtask selector. Header1320/directory1152bytes
только приffff; task records поselector+204, вспомогательные/FB ветви не закрыты.
Go um.ExtractN1TaskRecord: проверяет весь N1 и копирует точную запись;
ErrN1TaskNotFound,без перекодирования/FB dependencies/сети. Это record длясравнения,
не квалифицированный task payload. Проверены два native full N1 иcyclic wrapper.
1263f9a0 формируетread0306/0316 на4bytes; callbackBE decode, profile mask getters;
protected buffer helper не исполнялся.10 native functions вGit,516/516 checks,
um/transfer tests+vet прошли. [Writer и extraction](omron-task-writer-and-record-extraction.md),
raw research/hardware/20261118-cj1-task-writer-01. Далееstandalone task golden,
auxiliary/FB blocks испецификация task payload;qualification=false.

18 ноября фактический HEAD рабочей копии обнаруженdfae2637; последних
коммитов из сообщений (включая24f7fa8a) и raw-снимков нет. Не объявлять их
восстановленными или присутствующими. Текущий этап выполнен на этой базе.
Builder126519b0 создаёт44-byte task records(buffer+d0,count+d4), использует
SEC/PRG MakeTemporaryFilePath либо baseName.Left(len-3)+Format(%03X,taskID).
Consumer12610780 берёт record+4/file и+c/task argument для12611fd0→463/526;
full-program берёт отдельныйowner+78.126130a0 проверяет task flags/предупреждения,
writer не найден.17 listings в Git,435/435 static checks,transfer/um tests+vet.
[Таблица task-файлов](omron-task-file-table.md),raw:
research/hardware/20261118-cj1-task-table-01. Далее фактическая запись task bytes,
1263f9a0 и оставшиеся косвенные calls.Qualification=false,сети не было.

Найден marker/block writer12685840: plain CJ1 =48zero bytes + полный исходный
N1; protected flag→prefix word1=BE80 +380 bytes перед телом. В bit20-ветви
prefix256/magicCJ2X-or-CJ2M/marker81. Это разрешает offset4: префикс не raw N1!
Caller124c4310:124b9f60→126994d0→12685840→conditional metadata12694c50.
Go omron/um:WrapPlainN1Object,ParsePlainN1Object,ErrPlainN1ObjectLayout;
защита/CJ2/metadata не поддержаны,CLI пока не подключён,нет wire qualification.
Все Omron/research tests+Omron vet прошли;56static checks, native fixture в Git.
C: был полностью заполнен. Wrapperca45bdc1 теперь хранит Java/TEMP/TMP scratch
в Workspace/temp на F и восстанавливает env при success/failure. Go tests TEMP
тоже перенаправлялся на F. Системной очистки не было.
[Отчёт](omron-plain-object-envelope.md), raw:
research/hardware/20261117-cj1-protect-envelope-01. ДалееCLI/контрольный nativeOBJ,
сопоставление file+78 с буфером сервиса; .37READONLY,.36offline,сети не было.

Исправлена привязка адреса12612097: он в helper12611fd0, не непосредственно
в12610780; dispatcher вызывает helper по12610bbd. Это task download:
463/CDM_DOWNLOAD_TASK либо526/CDM_DOWNLOAD_TASK_EX, GetTaskConfigurationTable.
Full-program service3/532 — другая ветвь того же state8. PUSH214 в task helper
есть размер буфера532, не service532. Вызов12699520 здесь получает arg3=1.
XML inspector теперь --family task (taskServices), default program JSON сохранён.
12 tests,8 прежних program replay,8 новых task replay, CLI replay,51 static/replay
проверка прошли. [Отчёт](omron-task-versus-program-dispatch.md), raw:
research/hardware/20261117-cj1-dispatch-file-01.380-byte insertion/marker80-81
ещё не найдены; далее callers126994d0 и формирование полного файла+78.
Без сети/vendor execution, .37READONLY,.36offline.

Writer121c28ef прослежен: путь KBase+2ce8 / explicit path, N1 header330dwords,
1320bytes через N1WriteToArchive121c632d, big-endian serialization.
2296dwords двух compiler N1 воспроизведены арифметическим replay, не компиляцией.
Условный post-write121d3ef3 — именованный ConvertUMObjFromCJToULC, не общий
OBJ wrapper. Симметричная защита:126994d0(flag2bc)→1284f970→1284f9e0→
EncryptUMObject12859ac0→fileWrite128505d0. Вторая ветвь12699520 имеет callers
124da2b3/124e28da/12612097. Следом12612097, соединение со standard flag0,
место вставки380 bytes/marker80-81.55 проверок, без сети/vendor execution.
[Отчёт writer](omron-n1-writer-and-protected-file.md), raw:
research/hardware/20261117-cj1-obj-writer-01.

Дальнейший вызов12699880 прослежен до CEncryption::CompositeUMObject,
export12859af0→internal1285a430. Вычисляет файловую область base30/100 +380
при extended flag; length из header0c/08 либоdc/e0/e4, иначе fileLength-offset.
Машинная инструкция вычитает380 bytes; pointer-псевдокод Ghidra вводит в заблуждение.
При успехе обработанный buffer записывается обратно через CFile::Write.
Caller124c53e0 пропускает эту ветвь при flag0. Шестое слово входа0 даёт
native errorfffffffe; у обоих retained compiler N1 word5=0, но OBJ↔N1 ещё
не доказано. Не добавлять безусловное преобразование в backend.
Путь второй ветви начинается как CString-copy selected object+2ce8;
fallback+188 остаётся открытым.64 новых статических проверок прошли.
[OBJ workflow](omron-obj-protection-workflow.md), raw/manifest:
research/hardware/20261117-cj1-obj-flow-01. Следом OBJ writer/образец,
соответствие N1 и обычный путь без extended protection. Никакой сети/исполнения
vendor-кода; .37READONLY, .36offline.

Исследование источника IsExtendedProtect продолжено после переноса на F.
`12694ab0` читает big-endian uint32 по file offset4 и присваивает cached
PLC_KBase+2bc значение0/1 по равенству0x80 либо0x81 (descriptor+44 bit20).
Это прямой путь сброса и установки, **не измерение защиты ПЛК**. Заголовок
этого файла пока не сопоставлен с raw N1, применять правило к UM нельзя.
Caller124c53e0→12694ab0→CopyFileA(OBJ)→126995a0 читает380 байт по базе30/100,
затем24 байта; при текущем flag1 второе смещение увеличено на380.
12699950 обрабатывает380-byte хвост файла и в native workflow вызывает SetLength.
Мы vendor-код не выполняли и проекты этим способом не изменяли.
Тип второго setter доказан для ветви124a9da0→12674e10; fallback+188 ещё открыт.
79 статических проверок прошли. Отчёт
[источник флага](omron-extended-protection-origin.md), raw/manifest:
research/hardware/20261117-cj1-protect-origin-01. Следом формат OBJ↔N1,
12699880, источник пути12677730, fallback+188 и другие обновления свойства.
Бэкенд не менялся. Craft MCP недоступен, headless Ghidra работает.
.37READONLY, .36offline; сетевых обращений не было.

XML этап 17 ноября: StaticData/StaticRoot реально содержит скрытые StaticInfo.xml;
искать rg --hidden --no-ignore. Получены оба проекта × ETHERNET/SYSWAY × Local/Remote.
Оба наследуют записи3.0.0.0 (CDM_DOWNLOAD_PROGRAM) и532.4.1.0 (EX) изCS1Common/
VER4_0Common. Это РАЗНЫЕ CommandNo, не просто версии операции3! DriverReleaseVersion4.1.0
не firmware CPU. Новый переносимый inspect_cj1_services.py читает Cxlink subset,
case-insensitive physical lookup, provenance hashes/limits, без COM/network;
10 synthetic tests и8 snapshotreplay прошли, 21XML сохранён внеGit.
CX-P12613480 передаёт532, predicate126992e0 читает object+700/2bc;
установка флага/runtimeограничение версии ещё не прослежены. План0307 unresolved
уточнён (service3/0307 vs532/0317). Отчётomron-cj1-service-xml.md;
research/hardware/20261117-cj1-service-xml-01 snapshot/Ghidra/manifest.
READ ONLY, PLC не опрашивались. Следом setter2bc/реальный выбор +filebufferworkflow.

Следующий этап 17 ноября: обнаружено различие standard0307 и Ex0317:
ProgramDownloadExServiceCS1H_V4Common → CDownloadProgramExTransaction63018840,
encode630188f0 пишет03/17. В план0307 добавлена явная unresolved-причина,
ACK0307 тестом отклоняет0317. Первая транзакция ОБОИХ сервисов — точный
CReadStatusTransaction (RTTI63060210), не GeneralReadCommon. Её630385e0 после
status разбора читает sourcefile в буфер и проверяет UM-size; headerstamping
в этом фрагменте не найден, весь workflow ещё не прослежен. Генератор61a13580
читает XML Local/Remote Service с CommandNo/DllPath/ClassName/SuperClassName/
SupportVersion; selector61a13470 выбирает max<=requested, источник лимита для
CJ1 и конкретный XML ещё неизвестны. PlainLib62e01f90 только грузит выбранный класс.
28 статических проверок и transfer/CLI tests/vet/build прошли. Отчёт
omron-program-service-versions.md; raw/manifest вне Git
research/hardware/20261117-cj1-service-selection-01. READ ONLY, сети не было.

17 ноября, продолжение: проекты DEV CJ1H-H/CPU65 и CJ1M/CPU21 имеют PnP
категории, обе ведут в Common Files/Omron/Drivers/CXSDI_CS1Config.dll.
Legacy CDMPLC.DAT разобран: 50 групп, без отдельных CS1/CJ1; script202/category3
НЕ квалифицирован для CJ1. Найдены CS1Comms → SysWOW64/CS1BaseComms →
SeriesCS1MessageServices1.dll, ProgramDownloadServiceCS1Common и транзакция
63019140/630193a0: 0307, BE fields, align4, final8000, ACK count progression.
Добавлен чистый transfer.ValidateProgramWriteResponse: exact command/end/program/
offset/count, без сети/повторов, synthetic tests; scoped tests/vet/build прошли.
36 статических проверок, raw/manifest вне Git research/hardware/20261117-um-server-dispatch-01.
Отчёт omron-cj1-pnp-program-transfer.md. Следом: конкретный service/version selector,
sourcefile→transaction и standard/Ex, UM stamping; 256/64 остаётся открытым.
Никаких PLC обращений. .36 offline, .37 только чтение. Writer не добавлен.

17 ноября: статически прослежена цепочка CX-P `12610ab3` → CDM ordinal14
`CDMDownloadProgram` → `6001fe08` → ICXSCommunications slot14 →
cdmsvr20 `004395f9`; type5 ведёт к `0044c162` и общему `00431bcc`.
Это передача имени файла во внутреннем запросе, ещё не FINS-пакет.
Исправлен Ghidra exporter: listing body объединяется с high-function blocks;
на том же CX-P участке assembly вырос с135 до1430 инструкций и включает CALL.
23 статические проверки прошли. Отчёт `omron-cj1-native-transfer-path.md`,
raw/verification/manifest вне Git в `research/hardware/20261117-um-transfer-path-01`.
Следом: исследовать два server адреса, происхождение файла и CPU driver selector.
256/64 НЕ разрешено; writer не добавлен. ПЛК не опрашивались, READ ONLY остаётся.

Новый этап16–17ноября acc4после: word4 во4реальныхUM=ceil(bytes/64). APIServiceFields
добавляетblocks64/countMatches. Изолированные100bytes subcb_sum_make (CS2/Plain
побайтноодинаковы) выполнили80случаев безvendorDLL/сети; Goреплей38synthetic32bit.
Найдены0307handler/finalizerвCxPeriFnssrvCS2 иplain: последнийбит вызывает
zero-tail+sum+запись0/1/4 самимвиртуальнымCPU. НО residentcountquantum256,
аphysicalUMсоответствует64: отображениеещёНЕквалифицировано! NativeAlgorithmConfirmed
иwriteQualifiedfalse. Localcandidateизнашего4604byteCXP с3полями=ПОЛНЫЙисторический
readback SHA3d4c2e07..., исходники/ПЛКнеизменены. Этоисследовательскийрецептодногопроекта,
неwriterAPI. Корпусnative-um-sum-*+Ghidraсохранёнвum/testdata, rawworker/research
внеGit research/hardware/20261116-um-native-sums-01. Отчётomron-um-native-sums-and-finalization.md.
ДальшеCPUfamily/resident-wiremapping+native sender, ack/failuremodel. .36offline,READONLY.

Последний этап16 ноября — БЕЗобращений кPLC: words0/1 во всех4hardwareUM
точноравныlow/high16(sum32 bigendianwords[6:])); PythonиGo+bodymutationпроверены.
Word4НЕравенwidecarry, назначениеиnativeподготовка ещёнеустановлены. APIServiceFields
толькоdiagnostic, nativeAlgorithmConfirmed/writeQualified=false. Ghidra121c28ef
снова подтвердилнулевой330wordheader compiler path. Добавлен чистыйomron/transfer
и CLI plan-program-write:0307до996bytes(READ992!), finalbit15, точныеpayload/hash,
безendpoint/execute/Send и безheaderнормализации. Current2716=>996+996+724.
Отчёт omron-um-service-fields-and-write-plan.md; raw/replays/Ghidra/plan/manifest
внеGit research/hardware/20261116-um-service-fields-01. ДальшеnativeN1→sender,
word4, ack/obрывмодель, CPU/routeworkflow. READONLY, .36offline.

Стабильное окно16 ноября: все24 codewords текущего2716-byte CJ1H UM разобраны
в18инструкций с exact reencode. Добавлен DecodeTasks/inspect-image -decode-tasks
-model MODEL -timer-mode bcd: stop наunknown, весьtail сохранён, max65536инструкций,
профильявный, source/symbolnames/hardware=false. Производный recovered.cxt с
ASCII RecoveredTask256 собран безCX-One; веськод24words совпал, полныйобраз
различается только3servicewords и6packednamewords. Это не оригинальная LD/source.
2201 roots8000/8001 оба2301file device missing; symbols НЕполучены изCPU.
Отчёт omron-cj1h-task-decoding-2026-11-16.md; raw/IL/CXT/JSON/manifest внеGit
research/hardware/20261116-cj1h-stable-decode-01. .36offline НЕопрашивать, .37READONLY.

Последний CJ1H snapshot после занятия:2716-byte UM безFB, одназадача256/24codewords,
SHA d54d9fcb...; две копии совпали. Старый4604-byte readback теперь исторический!
Пользователь разрешил исследовать только .37; .36 offline, НЕ ОПРАШИВАТЬ.
Новые projects elevator-new4 CXP/OPT: 0502вернул CPU,ETN21(unit1),DRM21(unit2),
MAD42(SIO3). Две полные8012/1280words совпали с IOTable.Data нового И старого
CJ1H CXP после40-byte prefix и16-bit byte swap. Это REGISTERED table, не полная
независимая physical BasicIO проверка. Две8010/512words нулевые. Symbols только
из CXP, НЕ получены из CPU. Новый H компилируется4604bytes, отличается оттекущегоUM.
Отчёт omron-cj1h-new4-inventory-2026-11-16.md; raw/скрипты/manifest внеGit
research/hardware/20261116-cj1h-new4-inventory-01. READ ONLY, никакихwrites/modechanges.

Последующая ручная загрузка CJ1H16 ноября: оператор выбрал все доступные
сегменты, получил warning failed to download data (точный сегмент неизвестен).
Повторное ЧТЕНИЕ CJ1H дало две совпадающие4604-byte UM-копии, SHA3d4c2e07...
Независимая сборка неизменённого projects CJ1H CXP совпала во ВСЕХ словах,
кроме header0=29a8/1=a2be/4=48 против нулей. Все task/FB/config/directory/code
байты совпадают! Это уже подтверждённый source-to-live code match данного проекта;
полные файлы не идентичны, назначение трёх полей и warning НЕ объяснены.
Состояния до/после mode4/fatal0/nonFatal0. Другие сегменты не проверены,
никаких агентских записей/смен режима. CJ1M не перечитывался. Отчёт:
omron-cj1h-manual-download-readback-2026-11-16.md; raw/session/manifest вне Git:
research/hardware/20261116-cj1h-after-manual-download-01. Старые0FB/2880-byte
данные CJ1H ниже — исторические. hardwareQualified/writeQualified=false.

Offline-сверка16 ноября: оба projects CXP побайтно совпадают с tracked fixtures,
но содержат1FB/2instances/2tasks против живых5/9/1 и0/0/1. NewProgram1 имеет
ID257 в CXP и256 в живом CJ1H. Подробности/хеши: omron-hardware-source-comparison-2026-11-16.md.
В CompileCXPResourceBody/CompileCXPImage добавлена ветвь без FB: отсутствие
секций после полного аудита пустых instances/symbols, явная пустая карта IDs.
Полные2624-byte END-only файлы обоих CPU совпадают с archived native capsule.
Стендовые режимы/передача и соответствие живому исходнику не квалифицированы.

Аппаратный этап16 ноября: FINS/TCP identity/status, один W0 и две полные UM-копии
с каждого действующего CPU. CJ1M12380 bytes (5defs/9instances/1task), CJ1H2880
bytes (0FB/1task), пары SHA-256 совпадают. Исправлен ReadProgram: при0306/1104
уменьшать конечный запрос, но требовать явный last-block flag. Только чтение!
Состояния до/после mode4, fatal/nonFatal0. Оба UM приняты N1-инспектором;
состав отличается от исходных эталонов, соответствие текущим CXP не установлено.
Служебные начальные слова ненулевые; не угадывать их назначение/правила записи.
Отчёт: omron-hardware-readonly-2026-11-16.md. Сырые данные и manifest находятся
в workspace research/hardware/20261116-readonly-01 вне Git, передавать отдельно.

Актуальный операционный план: omron-hardware-verification-and-handoff.md.
Основной сценарий — управляемый Windows-стенд с начала, Linux параллельно.
Запасной Linux-first сценарий сохранён со статусом «Не реализован».
16 ноября пользователь подтвердил действующую установку: разрешено ТОЛЬКО
ЧТЕНИЕ. Оба CPU доступны по FINS/TCP на прежних IP/9600, identity/status
получены; доступ не разрешает запись, смену режима или снятие защиты.

Новый этап16 ноября: `compiler.CompileCXPImage` и CLI `compile-image` связывают
полный N1 из CXP/CXT и явного JSON настроек. Параметры: -model/-resource,
-timer-mode bcd, -settings FILE.json, -o NEW.n1. Образ совпадает с обоими4592-byte
эталонами для их исходных рецептов; неизменённые CXP дают4604 bytes. Никакие
исполняемые фрагменты не копируются из эталона. Добавлен containment-аудит
экземпляров относительно заданных банков. Существующие файлы не перезаписываются;
неоднозначный/неполный JSON отвергается. Первая full-image область: нулевые flags,
явный AC0/64/1536, nonempty scalar FB prefix, прежние ограничения task/body.
linked=true относится к N1; storageInitialized/globalStorageAudited/
sourceSettingsVerified/hardwareQualified=false. Следующее: provenance настроек,
инициализация/размещение и дальнейшая интеграция Nautilus/VSCode. Полная инструкция:
docs/design/omron-n1-build.md; пример JSON в omron/compiler/testdata.

Новый этап16 ноября: `codec.EncodeN1Header` генерирует42 фиксированных слова
из явных настроек и длин секций. Все42 совпадают с обоими сохранёнными N1.
Новый fb-memory-file-* содержит12 полных689-word N1 с ProbeFB, включая изменения
FB/stack. Каждый файл целиком совпадает с независимой Go-сборкой в тесте.
Причина прежнего Compile0: предупреждение Unreferenced Function Block Body;
обычный writer возвращает1/errorCount0 и пишет файл. Данные диагностики сохранены.
В writer передаётся тело LD0,BR3/OUT1,BR3; он сам добавляет FBBN/FBBRET.
Предварительная Compile не нужна: повторная компиляция может дублировать FBBN.
Ghidra21/22/23 закреплены хешами. Полный CXP linker ещё не подключён к этому
header API; следующим шагом соединить готовые части с явной моделью настроек.
Mapping CXP stack/options, initialization и hardware-приёмка остаются открытыми.

Новый этап16 ноября: `codec.EncodeN1FBMemory` вычисляет17 слов FB/stack-дескриптора
из явных настроек. Native header-memory-*12 наблюдений подтверждают адреса,
размеры, AC/D stack и отказ firstSize>totalSize. Default descriptor совпадает
с сохранённым FB header. Writer пишет его только при наличии определений FB;
12 полных файлов без определений побайтно проверены с нулевыми полями15..31.
Это не сборка header целиком. Изменённые дескрипторы ещё требуют полных FB-файлов.
Ghidra19/20 закреплены хешами. D0 действительно даёт10000, AC0 даётb200;
mapping CXP StackSettings к фактическому KBase остаётся открытым.
Попытки header-memory-02/03/04 создать минимальное определение дали Compile0,
не приняты как положительные эталоны; helper убран, raw сохранён вне Git.
Следующее: штатная инициализация определения/POU, затем новые полные FB-варианты.

Новый этап16 ноября: `codec.EncodeN1TaskFlags(profile, N1TaskOptions)` генерирует
пять подтверждённых битов header[6] из явных настроек. IR/DR=20000000,
binary timer=40000000, Extended TIM Hold=08000000, copy/overwrite=2000/4000.
Новые header-options-bcd/binary-* содержат64 штатных наблюдения: все комбинации
четырёх bool-настроек, два режима таймеров и два CPU. Каждый из656 слов каждого
N1 проверяется независимо; меняется только header[6]. Старые8 файлов также
проходят replay. Ghidra18 доказывает поля29c=IR/DR,2a0=BCD,2a4=Extended TIM Hold;
это снимает прежнюю неопределённость, но mapping этих настроек к CXP ещё открыт.
В текущем CXT нет прямо названных scalar-полей. Не выводить их из UI/defaults.
Полный global header, FB/stack memory setup, initialization и hardware не готовы.

Новый этап16 ноября: снят блокер штатного создания PLC_Program в библиотечном
oracle. MakeTaskConfigurationTable привязывает KBase; после GenerateInst,
AddToProgram и Compile удалось вызвать штатный полный N1 writer без GUI.
program-object-* содержит8 наблюдений (2 CPU ×2 программы ×2 состояния CII).
Go-тест независимо проверяет все656/658 слов каждого минимального N1, включая
header, configuration, directory и task record. Это исследовательский контекст,
не компиляция полного CXP. Его default flags=08000000 и нулевые FB/stack поля
отличаются от сохранённых проектов; production header по ним пока не выдаётся.
GenerateInst очищает CII; штатный SetCIIFlag(true) даёт код, совпадающий с Go.
GetProgramBinary возвращает end cursor. Ghidra15/16 закреплены хешами.
Следующий шаг: provenance настроек header, поля KBase29c/2a0/2a4, связь с CXP;
затем явная модель заголовка и полная независимая сборка с проверкой по oracle.

Последний этап: `CompileCXPTaskConfiguration` / CLI `compile-task-configuration
-model MODEL -resource PATH -timer-mode bcd FILE` генерирует288-word таблицу
конфигурации из header[42..329], полное совпадение с обоими N1. Unused slots
number|2000(<256) или number|1000(>=256); присутствующие задачи +8000,
OpStart1 +4000. Теперь SourceForTask поддерживает canonicalOpStart0/1,не только1.
Переключение startup очищает лишь4000 в table word, код/record/directory неизменны.
ResourceBody.TaskConfiguration выдаётся отдельно от post-headerWords;
compile-resource-body taskConfigurationGenerated теперьtrue. GlobalHeaderGenerated/
linked/storageInitialized/hardwareQualifiedfalse сохранены. Protected задачи закрыты.
Новые task-config-*48normal observations (3IDs x4startup x2protection x2CPU),
каждыйслучайнаfreshKBase/Program. NativeSetStartUpFlag12215fb8маскируетbit0,
Goотвергает-1/2/01. MakeTaskConfigurationTable121cbaed/Getter121cbecf:
number|9000|startup*4000|protection*c00. Getterabsent=0,ноfullwriterdefaultsненулевые!
Нативный defaultmemory snapshot7basepairs+GetStackRegisterAC0/64/1536иrawstarts
b200/b240,size600совпадаетсо17словамиheader15..31. General layout не квалифицирован.
Ghidra header-state-14 hash-pinned. Свежегоfullprojectwritercapture/hardwareнебыло.
Далее42fixedheaderwords:globaloptionflags/passwordconversion/profilemapping;
не копировать capturedheaderкакгенерацию. Полныйобраз пока не выдаётся.

Последний этап: `SourceForTasks` проверяет все программы ресурса, уникальные
TaskID/имена (casefold), IDs256..287. `CompileCXPTaskBlock` сортирует по ID,
выдаёт288-word directory + записи; смещения от начала каталога. Все652 слова
совпадают с обоими N1 (496..1147) для corpus source recipes, slots256/257=288/476.
Перестановка/разреженность ID, дубли/неверные counts/пустота/ошибки проверены.
SourceSpans/Generated block-relative, directory покрыт отдельным generated span.
`CompileCXPResourceBody` объединяет FBprefix+TaskBlock: все818 слов после330-word
header совпадают с обоими N1. Смещения фрагмента0/50/166 (defs/instances/directory).
Неизменённый CXP даёт821 слово, directory[257]=479 (другая первая цепочка).
Проверяет и сохраняет call links: instance name/ID -> generated definition/bodyID;
body/task number overlap пока отклоняется как неквалифицированный контекст.
CLI `compile-task-block -model MODEL -resource PATH -timer-mode bcd FILE`;
`compile-resource-body` дополнительно требует -number BLINK=0 и явный
-next-enable 0,0,0,0,0,0,0. FB-секции/задачи должны быть непустыми.
Report internalReferencesChecked/taskDirectoryGenerated=true;globalHeaderGenerated/
taskConfigurationGenerated/storageInitialized/linked/hardwareQualified=false.
Нет нового native capture: основание — прежний hash-pinned writer121c28ef и оба N1.
Далее global header/task configuration: не копировать старые330слов как новый
генератор. Все программы/FB-секции после заголовка уже генерируются из source.

Последний этап: `cxp.SourceForTask` / `compiler.CompileCXPTaskRecord` / CLI
`compile-task-record -model MODEL -program PATH -timer-mode bcd FILE` генерирует
цельную zero-flags запись задачи:32header +4entry +body +1return. Все188/176 слов
обеих задач совпадают с обоими N1 при восстановленной first-rung recipe; исходный
неизменённый CXP даёт191 слово первой записи. SourceSpans/Generated record-relative,
Code.Spans остаются body-relative, покрытие всех слов1 раз. Header[0]=ID,[1..8]=
ASCII по3байта/слово,[26]=length,[27]=32,прочее0. Вход0,bbc00f43,61440000+ID,
65440000; возвратbb800f48. Тело генерируется mixed API с separators/END.
Только IDs256..287, OpStart1, Protection.Enabled0, ASCII identifier1..24байт.
Other classes/IRDRprologue/protection/startup options ещё не квалифицированы.
Новые task-properties-*34наблюдения normal setters/getters+NOP: defaultID256,
descriptorCount288; SetTaskID принимает-1..289 (28успехов/6отказов), Go сознательно
отклоняет288/289 вне каталога. Setter защита отдельно от flags+200, которые0.
GetTaskType для256..287 =4. NativeGetName25 сохраняет25,writerусекает24,Goотказ.
Ghidra task-properties-13 hash-pinned. Это не новый полный native writer capture.
taskConfigurationGenerated/storageInitialized/linked/hardwareQualified=false;
следующее — каталог задач и global N1 header/linking, настройки/startup/initial data.

Последний этап: `CompileCXPMixedProgramBody(profile,doc,program,sectionMarkers)`
и CLI `compile-program-body ... -timer-mode bcd -fb-calls -section-markers FILE`
собирают все текстовые цепочки и одну подтверждённую7-cell scalar FB цепочку.
Все151 слово кода первой задачи совпадают с обоими N1 после восстановления
первой цепочки по portable oracle manifest: M LD W0.15/OUT W0.01, H LD W0.00/
OUT W0.01. Проверяются точные CXT хеши из manifest. Это старые captures,
не новая native компиляция и не усиление sourceIdentity operator-reported.
Исходный CXP содержит LD L1_Key_start/UP/SET ON и даёт154 слова; его первые5
слов заменяют2 слова эталонной цепочки. Все последующие149 совпадают.
Вторая программа через новую API сохраняет133/139 слов. Source/generated spans
и bindings перенесены со сдвигами, покрытие каждого слова ровно1 раз.
Только explicitBCD, не более1FBcall/программу; IL должен закрываться до FB и
конца каждой секции, даже без markers. Старые API без opt-in отвергают графику.
END обязателен/терминален, ошибочный source/context не выдаёт частичного тела.
Доказывает сборку151-word code вместе с FB, не task header/wrapper/полныйN1.
Следующее:32 слова task header +4 entry +1 return, связывание задач и image.

Последний этап: `SourceForFBDefinitions` читает все определения ресурса,
отклоняет неоднозначные имена/TaskID. `CompileCXPFBDefinitionSection` выдаёт
count+3zero+записи: все50 слов совпадают с обоими N1 (330..379). Номера записей
явно передаются map[точное имя]int, уникальны0..1023, не выводятся из TaskID.
CLI `compile-fb-definition-section -model MODEL -resource 'Resource[0]'
-timer-mode bcd -number BLINK=0 FILE`, повторять -number для каждого определения.
`CompileCXPFBPrefix` / CLI `compile-fb-prefix` с теми же флагами и обязательным
`-next-enable 0,0,0,0,0,0,0` объединяет определения/экземпляры. Все166 слов
совпадают с обоими N1 (330..495), относительные смещения секций0/50.
Проверены внутренние ссылки экземпляров по имени/пути, ошибки атомарны.
Перенумерация BLINK в7 не меняет FBBN/FBBCALL; источник с2 определениями и
TaskID1024/1025 даёт96 слов секции/212 всего и разные body-вызовы экземпляров.
Это изменённые Go-source тесты, не новые native full-project captures.
Пустые секции здесь отклоняются; их должен опускать будущий linker.
Основание заголовка — уже hash-pinned writer121c28ef, full-image branch.
N1 header/tasks, начальные значения, авторазмещение и hardware остаются открытыми.
instanceDefinitionReferencesChecked относится лишь к связи экземпляров с
определениями, не ко всем вызовам/версиям/задачам. linked=false сохранён.

Последний этап: `CompileCXPFBInstanceSection` / CLI
`compile-fb-instance-section -model MODEL -resource 'Resource[0]' -timer-mode bcd
-next-enable 0,0,0,0,0,0,0 FILE` генерирует всю секцию экземпляров. Все116 слов
совпадают с обоими N1 (380..495), две записи по54 слова начинаются со смещений
8/62. При удалении второго экземпляра/символа получается62 слова, count1.
Пустая секция отклоняется: её должен опустить будущий linker. Сохранённое
размещение и проверки предыдущего этапа сохранены; инициализации/полного N1 нет.
Семь слов заголовка: штатный SetNextEnableAddress121cf6e3, индексы0..6,
массив KBase+2d7c; конструктор121b96e5 обнуляет все7. Новые section-addresses-*
содержат74 наблюдения setter/readback/reset+NOP:70 успешных setter-вызовов,
4 отказа индексов-1/7. Нативный setter принимает ненулевые/отрицательные
значения — это не подтверждает их адресную семантику. Go/CLI требуют явно
нулевое состояние, не выводят его из размеров/CXP. Ghidra section-fields-11
и section-setter-12 архивированы с хешами. Поиск literal displacement —
кандидаты, не доказательство отсутствия алиасов. Свежего полного native writer
вызова не было. Далее: выбор next-enable из настроек, определения, задачи/linking
и initial storage. Linux execution и аппаратная проверка остаются открытыми.

Последний этап: `compiler.CompileCXPFBInstances` / CLI `compile-fb-instances
-model MODEL -resource 'Resource[0]' -timer-mode bcd FILE` генерирует все
глобальные записи с атомарной проверкой: взаимно-однозначная связь FB-символов
с POUInstances, точное имя/тип, [Auto]/A, отсутствие локальных FB/записей и
затенения глобальных экземпляров локальными именами. Проверяются пересечения
непустых T/BR0 и H/BR3 областей между экземплярами. Смежность допустима, порядок
памяти не обязан совпадать с исходным. Ошибки не выдают частичный набор.
Отчёт instanceIdentityAudited/instanceOverlapAudited=true, но
globalPlacementAudited=false: скалярные символы/операнды, все вызовы, задачи и
прочие ресурсы ещё не проверяются. Одиночная команда сохраняет прежний объём
проверки. Даже пустой набор требует корректной модели и BCD. Начальные значения,
новое размещение и полный N1 не генерируются.
Уточнение writer121c28ef: после счётчика экземпляров заголовок секции получает
7 слов из KBase+2d7c, отдельно от 7 размеров каждого экземпляра из program+f4.
Не считать их суммой размеров. В обоих N1 они нулевые; общий смысл не установлен,
поэтому новая команда выдаёт набор записей без заголовка секции.

Последний этап: `compiler.CompileCXPFBInstance` / CLI `compile-fb-instance
-model MODEL -instance 'Resource[0].GlobalVariables.POUInstances.Instance[0]'
-timer-mode bcd FILE`. Из CXP генерируются все54 слова записи экземпляра
(10 заголовок +44 runtime frame), полное совпадение двух экземпляров на обеих
моделях,17 служебных диапазонов. Поддержан интерфейс word,word,BOOL -> BOOL,
с EN/ENO и прежним офлайн-скалярным размещением. Абсолютные адреса берутся
из сохранённых BR-полей; новые адреса не выбираются, пересечения экземпляров
и все global symbol bindings не проверяются. Начальные значения не выдаются.
Отчёт явно показывает savedPlacementUsed=true, storageInitialized=false,
globalPlacementAudited=false, linked=false, hardwareQualified=false.
Эталоны instance-decode-* (4 штатных decode), instance-compile-* (110 Compile,
96 accepted/14 rejected) и ghidra-instance-envelope-10 hash-pinned. CII/CC
подтверждены штатными setters; LD FI/RSET FI/#SET ENO — целыми сохранёнными
буферами и статическим final pass. Новая компиляция целого экземпляра не
подтверждена: program-object probe завершён по таймауту до cases, helper удалён.
Далее — инициализация хранения, проверка размещения/идентичности символов,
заголовки секций, связывание задач и полный N1. Аппаратной проверки нет.

Новейший этап: `compiler.CompileCXPFBDefinition` / CLI `compile-fb-definition
-model MODEL -definition PATH -number 0 -timer-mode bcd FILE` генерируют из CXP
полную отдельную запись определения BLINK:23 слова кода с FBBN/FBBRET и23 слова
заголовка. Все46 слов совпадают с обоими N1. Номер записи явно задаёт вызывающая
сторона; TaskID исходника — отдельный номер FBBN. Не выводить один из другого.
15 исходных диапазонов +2 служебных охватывают23 слова кода. Счётчики портов
включают EN/ENO. Команда body по-прежнему выдаёт19 слов.
Новые эталоны codec/testdata/definition-wrapper-* (38 компиляций,14 успехов),
definition-properties-* (16 normal program setter/readback controls), сырая
Ghidra ghidra-definition-properties-09.json.gz. FBBN baredecimal0..2047, '&'
неприемлемо; номер записи0..1023; порты1..65 включая EN/ENO; ASCII-имя1..64.
Нативные setters допускают отрицательные счётчики и65-байтные имена; Go их
отклоняет, чтобы избежать некорректных значений и усечения. Заголовок flags0,
прочие варианты не генерируются. Writer layout проверен старым hash-pinned
fb-writer Ghidra. Далее — секция определений, ресурсы/инициализация экземпляров,
связывание задач. Полный N1 и аппаратная проверка ещё отсутствуют.

Самый новый этап: `compiler.PlanOfflineFBAllocation` и
`CompileCXPFBDefinitionBody` размещают офлайн-скаляры и генерируют19 слов тела
BLINK прямо из CXP на обеих моделях. CLI `compile-fb-definition-body -model MODEL
-definition PATH -timer-mode bcd FILE`. Таблица размещения и15 исходных диапазонов
в отчёте. EN/ENO bits0/1; BOOL по usage input/output/internal; INT/UINT/WORD после
округлённой BOOL-области; внутренние TIMER по BR0. Только нулевые онлайн-резервы,
без AT/retained/arrays/INOUT/прочих типов. Экземпляры/инициализация/обёртки/задачи
и N1 ещё не собираются. Полный19-словный результат совпал с обоими N1; перестановка
PV/PN и переход BOOL через границу слова меняют ожидаемые операнды.
Эталоны `omron/compiler/testdata/allocation-*`:40 настоящих COM-документов
PHeadDoc, все смещения/размеры совпадают с Go. Важно: нужен штатный event sink
для GetOnlineBOOLMemorySize/GetOnlineOTHERMemorySize с нулевыми ответами офлайн.
Без подписчика WORD-смещения ненадёжны. Ранние allocation-smoke-01/02 без sink
не считать доказательствами. Окончательный локальный run allocation-library-06;
проверены hash загруженного PHeadDoc и COM RVAs6ac0/73e0/eb60. Инструмент:
Invoke-CJ1AllocationOracle.ps1. Далее — обёртка определения, ресурсы/инициализация
экземпляров и связывание задач; дополнительные полные CX-проекты ещё нужны.

Последний этап: `codec.EncodeFBRelativeInstruction/Body` независимо собирают
короткие типизированные BR0/BR3-формы тела FB, смещения -2048..2047, TIMH в BCD.
Корпус `br-compile-*`:292 штатных InstructionInst::Compile (280 успешных,
12 отказов); Go точно совпал с218 наблюдениями в поддержанном объёме. Остальные
74 вне объёма: расширенные смещения, другие банки/синтаксис. Оба19-словных тела
BLINK пересобираются из уже размещённых типизированных ссылок. Аллокатора нет.
Ключ: нормальный Compile1216b432 с настоящим CStringArray и аргументами,
которые использует GenerateInst121e3bde. Сам SetUseFunctionBlockFlag1 проблему
не решает (`br-context-*`,72 прежних отказа). Сырые Ghidra и MFC ordinal map
зафиксированы в codec/testdata. Далее — перевод объявлений FB в размещение,
новые определения с переставленными/добавленными переменными и ресурсы экземпляров.

Новее: `codec.DecodeFBRelativeBody` независимо декодирует ограниченные формы
тела BLINK. CLI: `inspect-image -decode-fb-bodies -model 'CJ1H-H CPU65'
-timer-mode bcd FILE.n1`. Получает15 инструкций и типизированные ссылки BR0/BR3
с картой19 слов тела; относительные смещения знаковые12-битные. Эталоны
`codec/testdata/fb-decode-*` содержат12 штатных декодирований: исходник и пять
явных мутаций смещения на двух CPU. Сырые Ghidra-выгрузки также сохранены.
102 попытки обратного кодирования (`br-rendered-*`, `br-typed-*`) отклонены:
контекст самостоятельного OperandEncode был неполон. Теперь короткое кодирование
проверено через Compile (см. выше), аллокатор не реализован. Следующий шаг — перевод
переменных FB и режим knowledge base. Локальная выгрузка
`research/cj1-oracle/ghidra-fb-mode-05.json`: SetUseFunctionBlockFlag121cf742
через нормальный API устанавливает kbase+0x2a8 и вызывает121b0e14;
этот режим теперь проверен в `br-context-*` и `br-compile-*`.

Последний этап: `SourceForFBDefinition` и CLI `inspect-fb-definition` разбирают
шесть графических сетей BLINK в15 типизированных символических инструкций.
Порядок/инверсия и длины проверены по19 словам тела обоих N1 с маскированием
адресных полей. Это ещё не машинная компиляция определения и не аллокатор.
Эталон `omron/codec/testdata/fb-body-*` сохраняет24 наблюдения: programType32
не превращает физические H/T-операнды в относительные;16 форм отвергнуты.
Дальше исследовать штатный перевод/декодирование относительных операндов.
Локальные Ghidra-выгрузки: `research/cj1-oracle/ghidra-relative-bases-01.json`,
`ghidra-base-mnemonics-01.json`, `ghidra-fb-decode-01.json` (от корня cx-proger).
Результаты декомпиляции имеют неполные типы/ABI; сверять с ассемблером.

См. `omron/README.md` и `docs/design/omron-cj1.md`. Есть CXP/CXT parser/writer,
редактирование текстовой цепочки с сохранением графических FB, FINS UDP/TCP,
Host Link и Linux serial, чтение состояния/памяти/непрозрачного UM-образа,
read-only HTTP/SSE мониторинг и статический экстрактор фактов из cx_pdata.dll.
Теперь также есть независимый Go-кодек отдельных инструкций: обычная битовая
логика, MOV, шесть форм таймеров/счётчиков, ANDLD/ORLD, NOT, IL/ILC и LD/OUT TR.
Для шести входных инструкций и SET/RSET поддержаны @/% с прямыми адресами;
модифицированные OUT и TR остаются отклонёнными.
Добавлен `omron/compiler` и CLI `omron compile-rungs`: проверка структуры
выбранных текстовых цепочек, машинные слова, карта исходных строк и ошибки
без частичного результата. Для CLI нужны явные модель и пути цепочек; выборка
должна быть непрерывной внутри одной программы. Это ещё не N1 или PLC-пакет.
CLI разрешает точные имена из локальных/глобальных таблиц выбранной программы,
проверяет типы прямых привязок и выдаёт их происхождение. По девять исходных
символических цепочек на модель совпадают с целыми фрагментами задач N1.
Добавлены UP(521), DOWN(522), NOT(520):198 новых наблюдений по двум CPU и трём
режимам. UP занимает три контейнера, DOWN — четыре. Точное совпадение макроса
не раскрывает внутреннее поле0x120000 как адрес; другие его формы непрозрачны.
Первая исходная цепочка теперь компилируется, но изменена в сохранённом N1;
эталонными стали две другие исходные цепочки с UP. Следующий шаг — расширять
оставшуюся палитру и типизацию операндов, затем FB/задачи/полный образ.
Автоадресация, NUMBER, FB и неподдержанные инструкции не подменяются догадками.
Добавлена `compile-program-body`: строгая проверка объявленных секций/цепочек,
пустых цепочек и END. Все 41 текстовые цепочки каждого проекта компилируются
по отдельности; вторая программа целиком даёт 133 контейнера. Они совпадают
с исходными инструкциями задачи N1, но шесть дополнительных контейнеров
`b2400100,6140dead,6540dead` (дважды) остаются неизвестными. Они не вставляются
и не удаляются автоматически. Первая программа блокируется графическим BLINK.
Следующий предмет исследования: правило генерации этих фрагментов, пустые
секции и FB. Это всё ещё исходное тело, не собранная задача или N1.
Уточнение следующего этапа: шесть слов опознаны как два разделителя секций
`CMP(020) #DEAD #DEAD`. Ghidra подтверждает GetSLForSectionSeperator121cb520;
212 новых наблюдений подтверждают кодек CMP с двумя читаемыми операндами.
`compile-program-body -section-markers` теперь выдаёт все139 контейнеров Code
второй задачи в точном совпадении с N1 обеих моделей; Generated содержит
отдельные диапазоны служебных слов. Заголовки/обёртки задачи и полный N1 ещё
не генерируются. Пустые секции после первой и межсекционные IL отклоняются.
BLINK: семь объявлений PouVarList, шесть графических FBR, TaskID1024; CSV-ячейки
определения отличаются от DC1-ячеек вызова. Следующий шаг — их типизация,
размещение относительных операндов/таймеров и ABI вызова, затем сборка задачи.
Реверс FB продолжен: в N1 выделены определение BLINK (23 контейнера кода) и
два экземпляра по44 контейнера; целые буферы экземпляров совпадают с runtime.
`inspect-image -function-blocks` читает подтверждённую раскладку заголовков23/10
слов, сохраняет неизвестный относительный код и явно отклоняет другие варианты.
POUInstances T3072/H512 и T3074/H515 привязаны к пяти различающимся полям кода.
236 новых наблюдений внутренних FB-инструкций сохранены отдельно от публичного
кодека. У одного FBICALL между EncodeFunction и Serialize словоb2410806
меняется на927f0806; точное правило преобразования пока неизвестно. Функции
CreateTemporariesAndDoAsicFix121dea55 и DoAsicFixAndReturns121de87c выделены
в Ghidra как следующие исследовательские границы. Подробности и ограничения:
[формат FB](omron-fb-format.md). Генератора FB и полного N1 по-прежнему нет.
Следующий этап восстановил AddtoInvoke_Postamble1217170b: отдельный FBEND,
сброс CII и условная маска CC. Внутренний Go-пакет fbcode объясняет все девять
изменений старой трассы. 54 новых наблюдения проверяют примитивы; ещё четыре
вызывают штатные GenerateInvocationCode/GenerateEndOfInvocationCode на нормально
созданных объектах. Пролог/эпилог и счётчик инструкций подтверждены. Из пяти
уже закодированных команд тела сборщик воспроизводит все26 контейнеров вызова
BLINK в обоих N1; команды тела пока не получаются из CXP. Публичные компилятор
и кодек не расширяют область поддержки автоматически. Следующий шаг — типы
и графические порты, кодирование относительных передач и размещение переменных.
Передачи параметров теперь генерируются для скалярных BOOL/WORD/UINT/INT:
176 наблюдений AddOperandPreamble121749ea, count1, usage1/2. Проба
`-ParameterProbe` вызывает штатный метод и StringToDataType12175f7c без подделки
compiled-флагов. Материалы: codec/testdata/parameter-*; Ghidra в
internal/fbcode/testdata/scalar-*.json. Go ScalarTransfer выдаёт исходные typed
команды; их постамбула и четыре передачи эталонного BLINK совпадают полностью.
FBICALL/порядок портов ещё заданы явно. Следующий шаг: связать объявления и
графические пины CXP с этими типами, выбором/порядком передач; затем размещение.
INOUT/массивы/неявные EN/ENO и частные адреса этой операцией не поддержаны.

Добавлен cxp.PlanFBCall и CLI inspect-fb-call: исходный план семи ячеек вызова,
порядок PV/PN/IN и OUT по строкам/объявлениям, EN и адресные привязки, номера
экземпляра/определения, проверка версии. Числовые типы CXP сохранены сырыми.
Тест с явными транспортными типами и рецептом FBICALL совпадает со всеми27
словами LD+вызов для обоих образов. Это ещё не продуктовая компиляция графа.
Следующий шаг — доказать точную типизацию PouVarList и связать план с генератором;
потом тело FB/размещение. Только исходный семиячеечный формат поддержан.

Подтверждена PH_TYPE через PHeadDoc.dll (463eb440...) и цепочку сериализации
CX-P.exe (e91f15b8...): GetItem3 vtable0x1b0 -> row ctor1248a3b0 record+10 ->
writer1248a560 третья колонка. Полные материалы/хеши в cxp/testdata и fbtype_test.
Код1 — INT,0 — BOOL,2 — UINT,7 — WORD,13 — TIMER; native nDATA_TYPE — другая
битовая таблица. ScalarTransferForPin связывает исходные объявления с передачами;
176 native сравнений и27 слов LD+BLINK проходят без ручной транспортной карты.
Типы теперь именуются в плане; declarationTypesIdentified описывает имена,
compiled/typeCodesQualified остаются false до полной проверки проекта.
Следующий шаг: квалифицировать генерацию FBICALL и выбор invocation option,
затем включать ограниченное lowering вызова и продолжать тело FB/размещение.
GUI сейчас недоступен: node_repl сообщил failed to write kernel assets.
computer-use SKILL.md/документы прочитаны; Windows UI Automation не обходить.

Теперь FBICALL квалифицирован отдельным122-row корпусом codec/testdata/fbicall-*:
оба ID0..2047, не65535. Только каноническая decimal '&' форма; #0400 внутри
кодируется как decimal400. InstanceCall ограничивает ID и не считает их ёмкостью
контроллера; AssembleInvocation проверяет и raw-операнды FBICALL.
Найден настоящий caller в CX-P.exe:12713de0 ->1285adf6 ->IAT12b227f4 ->DLL export.
EBX=0 на12713e07, PUSH EBX на12713e40, CALL на12713e4c: false безусловно в этом
пути. Сохранён source-invocation-ghidra.json.gz; шире true всё ещё не квалифицирован.
CompileCXPFBCallBody / CLI compile-fb-call-body теперь генерируют27 слов LD+BLINK
из исходного проекта:6 source span,3 generated span,3 bindings, full image=false.
Это несвязанный фрагмент; определения/экземпляры FB не генерируются. Основной
компилятор всей программы по-прежнему отклоняет графику. Следующее: тело BLINK,
его относительные операнды, инициализация экземпляров/размещение, затем задачи/N1.
Стенд fbicall-library-01 был ошибочным: PowerShell '@('&'+i,'&'+d)' склеил строку.
Не использовать его. Итоговый fbicall-library-final имеет2 операнда и61 случай/CPU.

Таймеры требуют явного BCD/binary. Прямые DM-биты в обычной битовой логике CJ1
запрещены по руководству, даже если низкоуровневый стенд возвращает код.
**Компилятора полного проекта, полной палитры LD и развёртывания ещё нет.**
Текущие ограничения и переносимые эталоны: [кодек](../../omron/codec/README.md),
[нативное кодирование](omron-native-encoding.md),
[план](omron-revision-and-delivery-plan.md).

С 14 ноября 2026 года пользователь требует фиксировать дальнейшие этапы в коммитах.
Накопленная основа сохранена в `3a7a991c`, таймеры — в `987ff3f6`; последующие этапы
смотреть в Git log. Автор настроен только локально: Gregory Bednov,
gregorbednov@gmail.com. Не выполнять push без запроса; не терять работу reset/clean.
Первоначальная передача сохраняла всю рабочую копию поверх клона из Git bundle.
Манифест контрольных сумм находится в `E:\cx-proger\.transfer\manifest.json`.
Git origin первоначально указывает на локальный bundle — это снимок истории.

Исследовательский стенд теперь работает без GUI:
`tools/omron-research/Invoke-CJ1InstructionOracle.ps1 -Cases JSON -Output NEWDIR`
с необязательным `-TimerCounterMode bcd|binary`. Он использует собственный
ограниченный x86-процесс, штатные вызовы закреплённой хешем CX-Pdata.dll и Frida.
ProgramCheck, символы, FB, размещение и релокация в нём не выполняются.
Доступ к ПЛК не нужен; IP из проектов автоматически не использовать.
Изменения между Linux и Windows следует переносить явными коммитами/bundle;
пока исследования идут на Windows, считать эту рабочую копию основной.

## Среда

CX-Programmer: `C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-P.exe`,
версия файла `9.6.1.1`. Найден COM ProgID `Omron.CXP.KBase.KBaseComp2`,
CLSID `{AB322310-07E6-4D61-9FCB-10F19E7696FD}`. Это x86-компонент: для
исследования использовать также 32-битный PowerShell из `SysWOW64`.
Наличие регистрации COM ещё не доказывает доступность компиляции через него.

Переносимые инструменты размещаются в `E:\cx-proger\tools`. Подключение
окружения для терминала: `. E:\cx-proger\Dev-Environment.ps1`.
Кэши Go/GOPATH размещаются на E:, поскольку свободное место на C: ограничено.
`E:\cx-proger\projects` содержит исходные пользовательские CXP/OPT;
`research` — результаты статического разбора и справочник W342.

## Следующий эксперимент

1. Проверить версии и SHA-256 установленных `cx_pdata.dll`/`kbasecomp.dll`;
   сопоставить с исследованными хешами из `omron-cj1.md`.
2. Прочитать COM type information (без предположений о методах), выяснить
   поддерживаемые операции и условия лицензирования. Не обходить лицензию.
3. Открыть КОПИИ обоих исходных проектов в CX-Programmer, получить эталонные
   CXT и скомпилированные образы UM. Исходники сохранять неизменными.
4. Для минимальных проектов менять один признак за раз: LD/OUT/END, адрес,
   модификатор, ветвление, таймер, задача, FB. Сравнивать исходник, компиляцию и
   чтение обратно, фиксируя CPU/unit version и версии инструментов.
5. По подтверждённому формату писать независимый Go-код и тесты. Каталог из
   DLL содержит несколько семейств: 1504 записи/939 публичных написаний не
   означают поддержку всех этих инструкций CJ1.

Важное расхождение: CJ1M-проект объявляет CPU21, но заголовок его PLC Settings
остаётся CJ1H CPU65. Нельзя молча переносить эту конфигурацию на реальный CPU.
Тела FB и один вызов BLINK хранятся графическими элементами; импорт только SL
потеряет логику. По SSH пока не подключаться к IP-адресам ПЛК из проектов.

## Выполненные проверки Linux

`go test ./omron/... ./cmd/naut`, `go test -race ./omron/...`, `go vet`,
проверка Host Link через Linux PTY, ARM64-сборка и короткий CXP fuzz-прогон
(256964 входа) прошли. Созданный Go-кодом CXP независимо декодирован upstream
zlib blast: полученный CXT совпал побайтно. Это не аппаратная квалификация.
На реальных CJ1M/CJ1H код ещё не проверялся.
