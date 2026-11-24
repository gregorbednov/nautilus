# CJ1: default BR2 и условия поддержки FB

Продолжение: [masks и происхождение версий](omron-cj1-profile-version-origins.md)
связывает архивные descriptor masks с двумя library contexts, устанавливает
reset/default→current и отдельно сохраняет source stamp. Физическая unit version
и полный source→current путь не квалифицированы.

18 ноября 2026 года. Продолжение [пути UseDefault](omron-cj1-stack-source-origin.md).
Исследование выполнено статически в Ghidra 12.1.4 и через чтение PE, без
исполнения vendor DLL, запуска CX-One или обращения к ПЛК.

## BR2: отсутствие поля не случайно

В закреплённой CX-Pdata.dll таблица 122a6b18 содержит восемь строк по 12 bytes.
Для первых пяти строк читаются следующие литералы и третий DWORD:

| Индекс | Литерал | Размер из таблицы |
| --- | --- | ---: |
| 0 | T3072 | 1024 |
| 1 | C3072 | 1024 |
| 2 | DIFF8191 | 0 |
| 3 | H512 | 896 |
| 4 | H1408 | 128 |

Средний DWORD этих строк равен нулю; отдельная семантика ему здесь не назначается.
Инициализатор121ce7e9 использует таблицу на ветке с поддержкой FB, затем вызывает
преобразование адресов. Банк2 не берётся из сохранённого BR[2]:

- Writer FBMemory12439c30 пропускает индекс2:12439cdb CMP EDI,2 и следующий JZ.
- Reader12439de0 также пропускает2:12439e3e CMP EBP,2 и следующий JZ.
- Мост124b9be0 вызывает InitialiseBaseRegisters, затем setter для0..4,
  пропуская2 по124b9c1b/124b9c1e.

Таким образом, даже искусственно добавленный BR[2] не становится применённым
через эту цепочку. DIFF8191,0 имеет статическое происхождение как default-кандидат,
а не как восстановленное значение отсутствующего source-поля. Другие пути
изменения KBase и полное соответствие активному CPU-профилю здесь не доказаны.
Исходные проекты не редактировались, а генератор N1 продолжает получать
явные настройки; автоматического исправления BR2 нет.

## Поддержка FB и разные источники версии

InitialiseBaseRegisters вызывает `IsFunctionBlockSupported(true)`.
`UseFunctionBlockFlag` хранится отдельно по KBase+2a8. Его setter121cf742
проверяет `IsFunctionBlockSupported(false)` и при отказе принудительно сохраняет0.
Getter121cf79a только читает +2a8. Значение source-флага1 не доказывает,
что setter принял1 или что инициализатор выбрал FB-ветку.

Predicate121c0178 использует PLC pointer из KBase+2d50, capability masks
PLC+44/+68 и дополнительное состояние KBase+220. Часть семей имеет ранний ответ;
отдельные CP1L/CP1L-E проверки не переносятся на CJ1 по сходству названия.
На зависимой от версии ветке значение аргумента выбирает:

| Аргумент | Getter | Откуда читаются три bytes |
| --- | --- | --- |
| false | GetCurrentVersion121b0b4d | PLC+74/+75/+76 |
| true | GetDefaultVersion121b0bd5 | PLC+48/+49/+4a |

Оба getter сначала обнуляют результат и читают bytes только при соответствующем
family mask. Predicate собирает значение из первых двух bytes и использует
пороги0x0300/0x0200 в зависимости от другого capability bit. Полная таблица выбора
семейств, происхождение masks, текущей/default версии и всех caller пока
не квалифицированы. Эти внутренние bytes нельзя объявлять unit version
настоящего CPU, выводить из0501 internal system version или автоматически
переносить в `codec.Profile.UnitVersion`. В этом этапе CPU не опрашивались.

## Изменение аудита

`audit-image-settings` выводит `br2Selection=initialized-default-candidate`,
`br2DefaultCandidate={address:DIFF8191,size:0}`, исходный UseFunctionBlockFlag
без семантического повышения и `fbSupportProfileVerified=false`.
`bank-2` остаётся unverified как применённая настройка, независимо от наличия
и совпадения source BR[2]. `bank-2-default-table` сверяет явный рецепт с
закреплённой таблицей. Совпадение — passed только для таблицы; отличающийся
рецепт отмечается blocked. `fb-support-profile` сохраняет unverified.

Инициализация, sourceSettingsVerified, globalStorageAudited, hardwareQualified,
writeQualified и networkAccess остаются false. Вывод не разрешает загрузку,
не задаёт новые области памяти и не снимает остальные препятствия preflight.

## Проверка и передача

Корпус `omron/compiler/testdata/br2-profile-origin.json.gz` содержит прежние15
и новые5 функций. SHA-256:
`b6b6747cd2f8f740d483718eae0046a96b0cceeb78b93e925b85976817e8ee5b`.
`verify_cj1_br2_profile.py` воспроизводит предыдущие checkpoints и26 новых,
проверяет таблицу, exports и закреплённые PE-хеши. Локальный результат3327 checks
не является доказательством полноты caller или выбора конкретного CPU-профиля.

```sh
python -B tools/omron-research/verify_cj1_br2_profile.py omron/compiler/testdata/br2-profile-origin.json.gz --output NEW-REPORT.json
```

Переносимые Python-тесты не требуют vendor binaries. Проверены изменения
каждого нового checkpoint, другая строка таблицы и ложное повышение qualification.
Go-тесты проверяют отсутствующий/совпавший/противоречивый BR2, отличие recipe,
source-флаг0 и сохранение остальных неизвестных условий. Материалы:
`research/hardware/20261118-cj1-br2-profile-01`, включая Ghidra JSON/logs,
исходники, команды, тестовые отчёты и SHA-256 manifest.

Проверка завершена: все10 пакетов Omron, Omron CLI и vet прошли;6 Python replay
тестов прошли. Девять отдельных CLI-аудитов (4 исходных CXP и5 синтетических
вариантов) дали ожидаемые результаты с NAUTILUS_CLI=1. Начальные данные и
активный профиль не получили qualification ни в одном сценарии.

Далее: проследить загрузку capability masks и CurrentVersion/DefaultVersion
для двух конкретных CJ1-профилей; отдельно исследовать применение column5
FB-объявлений и состав начальных данных. Фактическая unit version, финализация,
полный backup и восстановление по-прежнему требуют отдельной квалификации.
