# Передача разработки Omron на Windows-стенд

Рабочая папка Windows: `E:\cx-proger\nautilus`. Linux-копия:
`/home/greg/cx-proger/nautilus`. Исходная база передачи:
`2db093e1e2e967a3a1b6b09a32fb575a84791748`.

## Что требуется сделать

Полноценный независимый бэкенд CJ1M/CJ1H: импорт, расширенный LD со всеми
родными инструкциями подходящего CPU, компиляция исполняемой программы,
загрузка/чтение обратно/верификация и мониторинг. Конечные инструменты работают
на Linux без CX-One, Windows, Wine или vendor DLL. Windows — исследовательский
эталон. Ethernet FINS и Host Link обязательны; online edit, Toolbus и полная
эмуляция CJ1 в Go VM отложены.

## Что уже реализовано

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
