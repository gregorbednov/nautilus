# CJ1: происхождение masks и разных версий контекста

19 November follow-up: [local numeric profile state](omron-cj1-local-profile-state.md)
now records default/current bytes[4,0,0] and IsExtendedProtect0 after SetPLC
on both selected library profiles. This resolves those local numeric observations,
not the physical unit version or current project protection state.

Продолжение: [ProgChk version word и backup CJ1H](omron-cj1-program-backup-review.md)
прослеживает один writer+3f8 как настройку program-check dialog, reset и default
fallback. Это не чтение physical unit version; отдельно снята свежая пара UM.

18 ноября 2026 года. Продолжение [FB-профиля](omron-cj1-br2-and-fb-profile.md).
Новых вызовов vendor DLL и обращений к ПЛК не было. Использованы статический
разбор закреплённых PE и ранее сохранённые события disposable library worker.

## Что установлено о masks

Архив `research/cj1-oracle/header-memory-01/events.log` содержит два profile
события после успешного SetPLC. Инструментация читала descriptor из KBase+2d50,
затем selector+44 и family masks+68/+6c. Точные значения:

| Library context | Selector | Low mask | High mask |
| --- | ---: | ---: | ---: |
| CJ1M / CPU21 | 8 | 0x400 | 0 |
| CJ1H-H / CPU65 | 8 | 0x80000 | 0 |

Это измерения старого локального native-контекста, не capability response
настоящего ПЛК и не текущий объект CX-One. Begin события явно имеют unitVersion=null,
programType=0, BCD, fullProgramCompiled=false. SHA-256 полного events.log:
`b2f1813d2a2a55d0270cb9ec88c715353f8afecd7c88f6490f19cf48b618c9a8`.
Портативная выборка: `omron/compiler/testdata/native-profile-observations.json`.
Проверка специально отвергает ошибочную подстановку 0x200 для CJ1H-H.

Статически SetPLC121ba36d вызывает FindPLCAndInitialise121ba4b2, который
перебирает зарегистрированные PLC descriptors, сопоставляет имена/CPU,
присваивает выбранный объект в KBase+2d50 и вызывает InitialiseBaseRegisters.
PLC constructor121b00b4 получает masks/selector аргументами и заполняет +68/+6c/+44.
Полный статический catalog initializer с конкретными аргументами для каждого
CJ1-объекта пока не восстановлен; соответствие двух строк выше опирается
на фактически сохранённые profile events, а не на предположение о каталоге.

## DefaultVersion и CurrentVersion

Constructor PLC сохраняет default bytes в +48/+49, обнуляет +4a и вызывает
InitializeCurrentVersion121b0a33. Для подходящего family mask эта функция
копирует +48/+49/+4a в +74/+75/+76. Числовые default-версии двух нужных
конструкторных записей здесь не установлены; masks не подменяют эти bytes.

SetCurrentVersion121b0aaa при packed значении0xffff вызывает такой же reset.
В другой ветке записывает первые два bytes в +74/+75 и ноль в +76. Поэтому
сохранённая default-версия и изменяемая current-версия могут расходиться.
В CX-P мост12674f00 вызывает этот setter, используя поля объекта +3f9/+3f8.
Сеть, источник заполнения этих полей, все caller и пути восстановления из
проекта/ответа CPU в этом этапе не прослежены. Функция не выполнялась.

GetCS2Version121b0cdb — ещё один API. При подходящих family masks он получает
CurrentVersion и упаковывает первые два bytes; для исключённых веток возвращает0.
Значит имя CS2Version само по себе не означает unit version реального устройства,
а принадлежность к этому API не устанавливает происхождение current bytes.

## Поле CS2SupportVersion в CXP

Во всех четырёх предоставленных CXP сохранено значение768 (0x0300). В просмотренном
начале reader124bbd20 поле читается в объект+cc с default0; ненулевое значение
выше0x0401 проходит в ветку проверки совместимости/предупреждения. Helper124bd190
копирует строку application object+138. Этот фрагмент не является вызовом
SetCurrentVersion и не доказывает восстановления unit version из данного поля.
Прямое связывание source stamp с current/default версией не реализовано.

Полная декомпиляция124bbd20 превысила30 секунд. Её не объявляем успешной:
сохранены исходный JSON, error и log; в переносимый корпус включён только
просмотренный asm-фрагмент124bbd4c..124bbd77, fullFunctionQualified=false.
Остальные9 новых функций успешно декомпилированы. Незакрытые части читателя
не принимаются как доказательство отсутствия косвенных путей применения stamp.

## Изменения независимого аудита

`audit-image-settings` теперь сохраняет `savedCS2SupportVersion` без перевода
в Profile.UnitVersion. `nativeProfileCandidate` содержит архивные masks, их
источник и хеш журнала; unitVersionKnown=false. Эти значения выбираются по
явно указанной модели рецепта, не извлекаются из source Config или устройства.
Весь fbSupportProfileVerified/sourceSettingsVerified/hardwareQualified/
writeQualified/networkAccess остаётся false. Проверка source-version-provenance
остаётся unverified, в том числе для изменённого/нулевого/отсутствующего stamp.

Это исследование помогает объяснить compiler context, но не разрешает запись
и не меняет профили кодека, алгоритмы N1 или инициализацию PLC-памяти.

## Воспроизведение

Корпус `omron/compiler/testdata/profile-version-origin.json.gz` содержит29
полностью декомпилированных функций с предыдущими этапами и1 просмотренный
asm-фрагмент. SHA-256:
`33d720b01557d44b6c10739ee136ce76604a4521cbd1ab8443d2fc804399db7d`.
Static replay закрепляет25 дополнительных checkpoints, masks архивных профилей,
строку source-поля и IAT setter. Локально прошло3393 checks. Число checks
не является мерой полноты caller, поддержки палитры или аппаратной приёмки.

```sh
python -B tools/omron-research/verify_cj1_profile_versions.py omron/compiler/testdata/profile-version-origin.json.gz --output NEW-REPORT.json
```

Материалы: `research/hardware/20261118-cj1-profile-versions-01`, с Ghidra reports,
отрицательным результатом полной декомпиляции, исходниками, архивной выборкой,
командами, тестовыми отчётами и manifest SHA-256.

Проверка завершена: все10 пакетов Omron, Omron CLI и vet прошли;9 Python replay
тестов прошли. Девять отдельных CLI-аудитов (4 исходных CXP и5 синтетических
вариантов) с NAUTILUS_CLI=1 сохранили неизвестную unit version и неподтверждённый
профиль. Дублированный stamp отвергнут без отчёта. Новых vendor/hardware вызовов нет.

Далее: источник fields+3f8/+3f9 в CX-P и конкретные default-version записи
каталога CJ1; отдельная source→current→support проверка. Затем — начальные
данные FB и аудит эффектов памяти. Unit version настоящего CPU, финализация,
полный backup и восстановление остаются условиями будущей аппаратной стадии.
