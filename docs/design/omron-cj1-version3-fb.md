# CJ1: FB и настройки при явном current3/current4 — 2026-11-19

Уточнение19 November: [версия проверки целой программы](omron-cj1-program-guard-version.md)
показала отдельную Program Check Version. Прежний Compile(...,false) выбирал
constructor FF/FF, хотя KBase current был3/4. Равенство файлов сохраняется,
но не квалифицирует само по себе version checks под3.0. Новый опыт задаёт
обе версии явно; production explicit UnitVersion по-прежнему закрыт.


Продолжение [сравнения обычных инструкций и N1 без FB](omron-cj1-version3-compile.md).
В 20 новых локальных запусках получены 1532 результата. Все совпали между
current3/current4 и с прежними retained native baseline. В том числе совпали
100 отрицательных результатов. Для 112 полных N1 файлов новый Go replay
дополнительно восстановил каждый word независимыми кодеками; 24 файла содержат
FB-определение. Сеть, настоящий ПЛК и CX-Programmer GUI не использовались.

## Серии и границы вывода

На каждую серию приходится две модели (CJ1M CPU21, CJ1H-H CPU65) и два явных
current (3.0, 4.0). Timer mode — только BCD.

| Серия | Входов | Результатов для двух моделей и двух версий |
| --- | ---: | ---: |
| BR compile, включая границы offsets/banks | 146 | 584 |
| FBBN/FBBRET wrapper и identifiers | 19 | 76 |
| Свойства определения, число портов | 8 | 32 |
| Компоненты экземпляра и CII/CC stages | 55 | 220 |
| Invocation skeleton | 2 | 8 |
| Scalar parameter transfer | 88 | 352 |
| Next-enable addresses, setter/reset | 37 | 148 |
| Header memory, без определения | 6 | 24 |
| Полный файл с определением и memory descriptor | 6 | 24 |
| Все 16 комбинаций четырёх header options | 16 | 64 |

Current setter выполняется после SetPLC, до SetUseFunctionBlocks и timer mode.
Getter/direct current fields проверены перед серией и после каждого case.
Default getter остаётся 4,0,0; local IsExtendedProtect — 0. Это воспроизводимый
путь установки current3 поверх выбранного библиотечного descriptor default4,
не измерение инициализации при default3.

Сравниваются целые case records: accepted/rejected, return codes, normalized
operands, words, flags, generated child lists, свойства, состояния памяти и
полные файлы. Исключаются только sequence, добавленное наблюдение current и
уникальный output path. Старые capsules ещё не записывали taskSelector;
новые отдельно проверяют selector65535 полного writer.

Из 1532 результатов 1432 имеют accepted=true, 100 — false. Это native coverage,
а не расширение production-палитры: среди native accepted есть формы за рамками
поддерживаемого независимым кодеком подмножества. Существующие baseline Go tests
продолжают проверять соответствующие реализации и их более строгие ограничения.
Новый Go test проверяет полное совпадение всех новых records с baseline и
напрямую реконструирует полные N1. Для этого не нужны DLL, Windows или ПЛК.

## Полные файлы и настройка памяти

112 файлов включают 24 no-FB memory cases, 24 FB memory cases и 64 header-option
cases. Memory cases содержат пять вариантов настроек и один rejected stack
split81/80: native setter оставляет явно восстановленное AC0/64/1536 состояние.
Этот fallback сохранён как наблюдение; независимый публичный encoder продолжает
отклонять некорректный вход, а не молча менять его.

В FB-файлах — task256 Probe с END и одно **неиспользуемое** определение ProbeFB:
LD0,BR3 / OUT1,BR3, directory/header/envelope, без instance section и вызова FB
из задачи. Нормальные методы создают и привязывают объекты; байты/flags объектов
не подменяются. У definition один Compile с результатом0, errors0/warnings1,
сообщение `0:Unreferenced Function Block Body - ProbeFB`. Full writer возвращает1,
его error array пуст. Эти результаты и диагностическое сообщение совпали
с baseline; return0 здесь не означает неуспех создания файла.

Go восстанавливает program/definition words, optional FB section, все42 header
words, task configuration, directory и record wrapper. Все файлы сравниваются
побайтно. Для FB scope присутствует 17-word memory descriptor, без definitions
он отсутствует независимо от getters. Header options воспроизведены кодеком.
Различий current3/current4 в этом наборе нет.

## Инструменты, проверка и сохранение

`Invoke-CJ1InstructionOracle.ps1 -LocalCurrentVersion 3.0|4.0` теперь допускает
проверенные single-context FB compile/definition/instance, invocation, parameter
и section probes. Для них требуется BCD. Второй контекст task configuration,
decode, profile-state и другие нерассмотренные probes по-прежнему отклоняются.
Четыре negative launcher controls (binary FB, пропущенный timer mode, второй
context и decode) завершились до создания output directory/worker.

Исходный `instruction-oracle.js` и вставка current-version не менялись;
их hashes совпали с предыдущим этапом. В каждой новой capture сохранены
base/insert/result, три единственных anchors, injected source/request,
provenance11files и raw events. Legacy provenance.programType остаётся0:
фактический programType0/16/32 проверяется по begin и injected request,
а не переинтерпретируется из этого старого поля.

Portable fixture: `omron/codec/testdata/native-version3-fb.json.gz`.
`TestExplicitCurrent3FBSettingsAgainstQualifiedBaseline` сверяет retained
baseline и независимо реконструирует 112 N1. Baseline capsules и manifest
hashes сохранены; предыдущие файлы не перезаписывались.

`verify_cj1_version_fb.py` проверяет 20 runs, inputs/baselines, versions,
masks, порядок, return codes, поля и целые файлы. С `--capture-root` проверяет
raw logs, provenance files, реконструкцию transformed observer, injected request
и saved N1. Полный replay дал 4222 проверки.

```text
python -B tools/omron-research/verify_cj1_version_fb.py --repo . --capture-root C:/cx-proger/research/cj1-oracle
go test ./omron/...
python -B -m unittest discover -s tools/omron-research -p "test_*.py"
```

Все10 Go Omron packages, `go vet ./omron/...` и 47 Python tests прошли.
Общий cmd/naut набор повторно не запускался: его отдельные Windows failures
и timeout описаны в [предыдущем отчёте](omron-cj1-version3-compile.md).

Новые raw пакеты под `C:/cx-proger/research/cj1-oracle`:
`version3-fb-{group}-v{40,30}-20261119-01`, где group — десять строк таблицы
(машинные имена записаны в hardware runs.json). Журналы, исходники и manifest
index — `C:/cx-proger/research/hardware/20261119-cj1-version3-fb-01`.
Каждая завершённая директория имеет SHA256SUMS. Это внешние runtime evidence,
не новые static native evidence IDs Craft. Frozen пакеты предыдущих этапов
не менялись.

## Следующий шаг

Нужно связать отдельные успешные компоненты в целый current3 FB-проект с
**используемым** определением, instance section и реальным вызовом из задачи;
сравнить native full image с независимой source compilation и проверить
версионные ограничения за рамками этих boundary tests. Неиспользуемый ProbeFB
не доказывает эту цепочку. Explicit production Profile.UnitVersion3.0 и binary
FB пока закрыты; автоматического выбора профиля по manufacturing read нет.

Затем остаются protection/service0307/0317, CPU finalization0/1/4,
real64/resident256, prefix и согласованный write/readback/original-return
опыт. Protected:No, local flag0 и support predicate не приравниваются.
HardwareQualified/WriteQualified остаются false; новых действий оператора
для следующего локального исследования сейчас не требуется.
