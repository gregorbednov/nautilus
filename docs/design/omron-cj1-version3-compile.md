# CJ1: инструкции current3/current4 и binary N1 без FB — 2026-11-19

Уточнение19 November: [версия проверки целой программы](omron-cj1-program-guard-version.md)
показала отдельную Program Check Version. Прежний Compile(...,false) выбирал
constructor FF/FF, хотя KBase current был3/4. Равенство файлов сохраняется,
но не квалифицирует само по себе version checks под3.0. Новый опыт задаёт
обе версии явно; production explicit UnitVersion по-прежнему закрыт.


Продолжение [локального опыта current3](omron-cj1-version3-library-profile.md).
Нативный oracle и независимый компилятор совпали на 5560 результатах кодирования
и 16 полных файлах N1. Добавлена сборка binary-образов без FB через API и CLI.
Это локальное сравнение компиляторов; обращения к ПЛК и записи программы
на этом этапе отсутствовали.

## Контекст и измеренный охват

Использована закреплённая CX-Pdata.dll, SHA256
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`,
в собственных скрытых disposable x86 workers. CX-Programmer GUI не запускался.
Для каждого профиля: SetPLC, явный режим BCD/binary, затем
SetCurrentVersion(3,0,0) либо (4,0,0). Default остаётся 4,0,0; getter и поля
current проверяются перед серией и после каждого case. Это не эксперимент
с инициализацией descriptor при default3.

Из 1456 различных пар mnemonic/operands существующих instruction corpus
независимый кодек принимает на обеих моделях 689 форм в BCD и 701 в binary.
Именно это пересечение отправлено oracle; отброшенные формы не объявляются
поддержанными или запрещёнными нативной библиотекой.

| Данные | Модели | Local current | Режимы | Результат |
| --- | --- | --- | --- | --- |
| 689 BCD форм | CJ1M CPU21, CJ1H-H CPU65 | 3.0, 4.0 | BCD | 2756 совпадений слов |
| 701 binary форм | те же | 3.0, 4.0 | binary | 2804 совпадения слов |
| Два целых образа | те же | 3.0, 4.0 | BCD, binary | 16 побайтных совпадений |

Образы — собственные `bit-full` (2632 bytes) и `boundary-2824` (2824 bytes),
с текстовыми LD/OUT/NOP/END, без FB, с CII/section markers и terminal END.
Проверяется результат компиляции и native file method с selector65535,
returned1/errorCount0, а затем весь файл, включая header, task configuration,
directory, record wrappers и инструкции. Это два небольших проекта,
не компиляция всех пользовательских проектов и не исполнение таймеров.

Во всех четырёх сочетаниях модели и образа версии current3/current4 дали
одинаковые файлы. BCD/binary различаются только словом заголовка6 (byte24):
`00000000` против `40000000`.

| Образ | BCD SHA256 | Binary SHA256 |
| --- | --- | --- |
| bit-full | `798dca90199c45dee2c55711d6d9685fba826ae7cb1c66fa98e2bbcb6a76b0f8` | `87ed301503b374ed2025377ade99897b5d6877e6e5f68d410258b03d3ff2a87a` |
| boundary-2824 | `ce500168349fadf708c2d07dad3c4eb32dde99d710220c920190cfc1154de7be` | `c96592f79747caa284dfe1dbb2bd3db260bb5a448df8a8dd93572fe891ba8ecb` |

## Исправление независимой сборки

Прежде полная binary-сборка останавливалась на BCD-only сериализаторе
FB memory descriptor даже при отсутствии FB. Теперь профиль и supplied memory
settings по-прежнему валидируются, а descriptor сериализуется только при
наличии definitions. Пустые instance collections проходят полный source audit;
сохранённые экземпляры, определения, FB-типизированные переменные и графические
вызовы нельзя игнорировать ради binary-сборки.

`CompileCXPMixedProgramBody` допускает текстовый binary-код, сохраняя отказ
для binary FB graphics. `VerifyCXPImageBuild` воспроизводит binary-образ.
CLI `compile-image` принимает явный `bcd|binary`; при отказе output не создаётся.
Binary FB definitions/instances и `EncodeN1FBMemory(binary, ...)` остаются
закрытыми. Ограничения zero options и stack AC0/64/1536 сохранены.

Пример для собственного проекта без FB, из корня репозитория:

```text
naut omron compile-image -model "CJ1M CPU21" -resource "Resource[0]" -timer-mode binary -settings omron/compiler/testdata/n1-finalization-probe-settings.json -o NEW.n1 omron/compiler/testdata/n1-finalization-probe.cxt
```

## Воспроизводимость

Существующий `instruction-oracle.js` не изменён, SHA256
`35a38b6f45cd961344ded9742c971306f4615f3d619809cef9c71f66a7c58c63`.
Опциональный `-LocalCurrentVersion 3.0|4.0` создаёт отдельную копию из pinned
base и `current-version-compile-insert.js`, проверяет три единственных anchors
и сохраняет base/insert/result hashes и request. Несовместимый probe,
отсутствующий timer mode и неизвестная версия отклоняются до создания worker
и output directory. Отдельный legacy NOP smoke подтвердил неизменный default
путь: unitVersion null, семь provenance files, исходный observer без вставки.

Portable corpus `omron/compiler/testdata/native-version3-compile.json.gz`
содержит 12 runs, исходные requests/events/provenance/transforms и независимые
слова/образы. `version3_oracle_test.go` заново компилирует инструкции и файлы
из CXT на Go; Windows, DLL и сеть для replay не нужны.

`verify_cj1_version_compile.py` проверяет scope, identity, masks, timer mode,
current/default, sequence/order, words и целые образы. С `--capture-root`
он также проверяет журналы, 11 provenance files каждого run, saved native N1,
восстанавливает transformed observer и сверяет инжектированный request.
С `--repo` проверяет неизменность исходных JS. Полный replay: 17352 проверки.

```text
python -B tools/omron-research/verify_cj1_version_compile.py --repo . --capture-root C:/cx-proger/research/cj1-oracle
```

Raw captures расположены под `C:/cx-proger/research/cj1-oracle`:
`version3-code-{bcd,binary}-b{0,1}-v{40,30}-20261119-01`,
`version3-images-{bcd,binary}-{4,3}-20261119-01` и
`version3-launcher-legacy-smoke-20261119-01`.
Журналы подготовки, первоначальный binary отказ, reference helper, новые
source snapshots и отчёты —
`C:/cx-proger/research/hardware/20261119-cj1-version3-compile-01`.
Каждая завершённая директория имеет отдельный `SHA256SUMS`; старые frozen
пакеты не переписывались. Это внешние runtime evidence, не новые native
evidence IDs статического Craft analysis.

## Остающиеся условия

`Profile.UnitVersion="3.0"` по-прежнему отвергается: текущий corpus доказывает
равенство конкретных форм и двух файлов, а не квалификацию всей палитры,
FB/init/base-register решений и версионных запретов. Автоматической подстановки
версии из PLC Information нет. Следующий опыт — отдельные current3 FB/settings
эталоны и граничные/запрещённые формы с тем же контролем local context.

Legacy event/provenance `fullProgramCompiled:false` сохранён как исходный
scope-флаг инструмента; факт получения конкретного полного N1 устанавливают
case.programObject.file и побайтное сравнение. Новый корневой флаг
`fullUserProjectsCompiled:false` явно ограничивает вывод.

Protected:No, local IsExtendedProtect0 и predicates не приравниваются друг
к другу. Выбор0307/0317, CPU finalization0/1/4, real64/resident256, prefix
и согласованный write/readback/original-return опыт остаются открытыми.
HardwareQualified и WriteQualified — false.

## Проверки реализации

`go test ./omron/...` прошёл для всех 10 пакетов; новый replay compiler
проверяет 5560 кодирований и 16 образов. `go test ./cmd/naut -run '^TestOmron'
-count=1 -timeout=120s` прошёл, включая binary CLI и отказ FB без output.
`go vet ./omron/...` и все 45 Python research tests прошли.

Дополнительно запущенный общий `go test ./cmd/naut` не прошёл на Windows:
`TestBuiltControllerActivatesWarm` ищет executable без `.exe`, manifest-tag
tests используют `aux.st`, golden diagnostic различается разделителем пути.
Затем `TestCheckReportsStructureWarnings` завис в WriteFile при захвате вывода
и сработал общий timeout10m. Это проверки вне изменённого Omron-пути;
они не исправлялись в этом этапе. Результат общего набора не объявляется
успешным. Краткий validation report сохранён в hardware-пакете.


Продолжение: [FB/settings current3/current4](omron-cj1-version3-fb.md) —
1532 результата и 112 полных N1, включая 24 с неиспользуемым определением.
Следующая граница — целый FB-проект с instance section и вызовом из задачи.
