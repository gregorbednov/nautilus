# CJ1: local current3 и predicates библиотеки — 2026-11-19

После [подтверждения реальной Unit Version3.0](omron-cj1-manufacturing-read-2026-11-19.md)
снят отдельный локальный опыт закреплённой CX-Pdata.dll. Он устанавливает,
как изменить current-версию выбранного descriptor и какие результаты дают
конкретные проверки поддержки. Production-кодек и правила передачи не менялись.

## Измерение

Для каждого профиля создан новый KBase в собственном disposable x86 worker.
Выбор через SetPLC, вызов SetBCD, затем три состояния:

| Модель | Состояние | Default getter/fields | Current getter/fields | FB current/default | Local IsExtendedProtect | NOP |
| --- | --- | --- | --- | --- | --- | --- |
| CJ1M CPU21 | default | 4,0,0 | 4,0,0 | 1/1 | 0 | A0000000 |
| CJ1M CPU21 | current3 | 4,0,0 | 3,0,0 | 1/1 | 0 | A0000000 |
| CJ1M CPU21 | reset | 4,0,0 | 4,0,0 | 1/1 | 0 | A0000000 |
| CJ1H-H CPU65 | default | 4,0,0 | 4,0,0 | 1/1 | 0 | A0000000 |
| CJ1H-H CPU65 | current3 | 4,0,0 | 3,0,0 | 1/1 | 0 | A0000000 |
| CJ1H-H CPU65 | reset | 4,0,0 | 4,0,0 | 1/1 | 0 | A0000000 |

`current3` вызывает `PLC::SetCurrentVersion(3,0,0)`, reset — тот же setter
с255/255/255. Getter и прямые fields сравниваются отдельно:
default+48/+49/+4a, current+74/+75/+76. Поддержка FB измерена через
`IsFunctionBlockSupported(false/true)`, то есть current/default ветви.
True predicate не заменяет компиляцию/исполнение FB.

GetPLC_Name вернул именно `CJ1M` и `CJ1H-H`; aliases не предполагались.
Selector8, low masks400/80000, high mask0 согласуются с предыдущими
[локальными наблюдениями](omron-cj1-local-profile-state.md).
NOP инициализируется с результатом1; encoder возвращает0 и одно слово
A0000000 во всех шести состояниях. Значение return code не объявляется
универсальным правилом успеха других методов.

## Predicate расширенной защиты

В каждом состоянии `IsExtendProtectionSupportedUnitVersion` вызван с packed
версией0300/0400/0401 и вторым аргументом0/1 — всего36 локальных вызовов.
Во всех этих точках он вернул0 для обоих descriptor.
Смысл второго аргумента здесь не назначается по предположению.

Статическая функция121b1dfe содержит сравнения имён `CS1G-H`, `CS1H-H`,
`CJ1G-H` и порог0401 в этой ветви. Литерал `CJ1G-H` нельзя автоматически
подменять на `CJ1H-H`: фактический GetPLC_Name последнего именно CJ1H-H.
Наблюдения не дают правила «повышение до4.1 разрешает extended protection
на CJ1H CPU65». Они также не квалифицируют все возможные версии и модели.

Поддержка формата, локальный IsExtendedProtect свежего KBase и показанный
пользователем `Protected: No` — разные наблюдения. Их эквивалентность
не установлена. Нельзя переключать0307/0317 только по этой таблице.

## Реализация опыта и переносимость доказательств

Новые research tools:

- `Invoke-CJ1VersionProfileOracle.ps1` — фиксированный опыт двух моделей,
  без входа произвольных инструкций/версий; проверяет SHA256 DLL/injector,
  новый output directory, deadlines, завершение/очистку собственных процессов;
- `version-profile-oracle.js` — проверяет шесть именованных exports/RVA,
  снимает восемь событий begin/state×6/done и кодирует только NOP;
- `verify_cj1_version_profile.py` — полностью офлайн проверяет retained events,
  порядок, scope, versions/getters/fields, masks, predicates, NOP и reset.

Новый corpus `omron/compiler/testdata/native-version3-profile.json`
работает без Windows/DLL. Дополнительные `--capture`, `--repo`, `--binary`
сверяют исходный журнал, provenance/files, текущие sources, полный SHA256
и exports PE. Windows-путь в request разбирается переносимо.
Предыдущие probes, их snapshots и runtime Go-профили не изменялись.

```text
powershell -NoProfile -ExecutionPolicy Bypass -File tools/omron-research/Invoke-CJ1VersionProfileOracle.ps1 -Output NEW-DIRECTORY
python3 -B tools/omron-research/verify_cj1_version_profile.py
python3 -B -m unittest discover -s tools/omron-research -p 'test_*.py'
```

DLL SHA256: `a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
Raw event log SHA256:
`34bb6dbdcb29a5f97aee6263af9af307225cffe07ef163f97d7a8c227cab8fdf`.
Fresh worker закрывается после опыта; GUI и сетевые API ПЛК не вызываются.
Это был запуск vendor-библиотеки в нашем процессе, а не только статический
анализ. Новых аппаратных запросов или записей не было.

Полный retained replay дал133 проверки; все43 Python-теста прошли.
Отрицательные launcher controls: существующий output и неверный injector
отвергнуты до запуска worker; для второго output не создан.
Go source не менялся, Go-тесты не повторялись. Предыдущие10 пакетов/vet
и аппаратный manufacturing read остаются отдельными результатами.

Frozen пакеты под `C:/cx-proger/research`:

| Directory | Files excluding SHA256SUMS | Manifest SHA256 |
| --- | ---: | --- |
| cj1-oracle/version-profile-20261119-01 | 11 | `f8d2252959a64d4fecabe9b27fc3deb6fbd3a73d79ecd688fec1ea023617ee9d` |
| hardware/20261119-cj1-version-profile-01 | 26 | `ffea670ade7dda94054528b8d852a4afb4b1a5b217c4222a702eb35534389b0f` |

Статическая часть отражена в Craft finding fnd_6; native runtime journal
сохранён отдельно и не выдаётся за выполнение Craft static tools.

## Следующий gate

Наблюдение setter3.0, predicates и одного NOP ещё не квалифицирует всю
палитру, настройки FB/base registers и целый образ с explicit UnitVersion3.0.
Инициализация KBase выполнена до смены current; default остался4.0.
Нельзя заявлять, что все решения инициализатора были выполнены при default3.0.

Следующее: нативные instruction/full-image эталоны с явно установленной
current3.0, сравнение с независимой компиляцией и проверка версионных запретов.
Только после этого расширять допустимые explicit codec profiles для
подтверждённого набора. Пока `Profile.validate` продолжает отвергать любую
непустую UnitVersion; автоматической подстановки неизвестной версии нет.
Защита, CPU finalization0/1/4, real64/resident256, prefix и полноценный
write/readback/original-return опыт остаются отдельными условиями.
FullProgramCompiled/ExplicitCodecUnitVersionQualified/HardwareQualified/
WriteQualified остаются false для этого локального опыта.


Продолжение: [инструкции current3/current4 и полные N1](omron-cj1-version3-compile.md)
подтверждает 5560 результатов и 16 образов, добавляя binary-сборку без FB.
Полная explicit UnitVersion3.0 и binary FB остаются отдельными условиями.
