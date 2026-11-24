# CJ1: измеренное локальное состояние профиля библиотеки

19 ноября2026 года. Завершение этапа, прерванного переносом на C:.
В закреплённой CX-Pdata.dll измерены ранее неизвестные численные default/current
bytes выбранных CJ1-профилей и локальный IsExtendedProtect после SetPLC.
Независимый Go-аудит сохраняет эти наблюдения отдельно от аппаратных свойств.

## Результат измерения

| Выбранный профиль | Selector | Family mask low/high | Default bytes | Current bytes | KBase IsExtendedProtect |
| --- | --- | --- | --- | --- | --- |
| CJ1M / CPU21 | 8 | 0x400 /0 | [4,0,0] | [4,0,0] | 0 |
| CJ1H-H / CPU65 | 8 | 0x80000 /0 | [4,0,0] | [4,0,0] | 0 |

Это bytes **локального descriptor**, не ответ настоящего CPU. Они не переводятся
в Profile.UnitVersion и не устанавливают firmware/unit version контроллера.
В частности, internal system version0501, source CS2SupportVersion и это
локальное current/default состояние остаются разными источниками данных.

Собственный disposable x86-worker создал один KBase, последовательно выбрал
CJ1M и CJ1H-H через SetPLC, явно включил BCD и снял поля после каждого выбора.
Между profile events штатно закодирован обычный NOP, оба результата приняты.
CXP не загружался, полный проект не компилировался; сетевые API ПЛК и GUI не вызывались.
Измерение не проверяет сброс ранее установленного flag1 при SetPLC и не
доказывает, что новый KBase создавался отдельно для каждого профиля.

## Смещения и воспроизводимость

Источник: CX-Pdata.dll SHA256
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.

- descriptor получен из KBase+2d50;
- default bytes: descriptor+48/+49/+4a;
- current bytes: descriptor+74/+75/+76;
- IsExtendedProtect: KBase+2bc.

Эти смещения связаны с предыдущим статическим разбором
[default/current версий](omron-cj1-profile-version-origins.md) и
[именованного getter защиты](omron-extended-protection-dispatch.md).
Offline verifier повторно проверяет hash PE и конкретные машинные bytes:
getter120126ca читает+2bc;121b0a88/121b0a94/121b0aa0 копируют три default bytes
в current. Сам verifier не запускает DLL и не обращается к сети.

Исходный журнал событий SHA256:
`b3a4139b313dfdc0a240c7d288bb80a4da8c6824f030d6499e7e25c069a65a98`.
Все6 событий включены в переносимый
`omron/compiler/testdata/native-local-profile-state.json`: begin,
profile/case для двух моделей и done. Begin явно имеет unitVersion=null,
fullProgramCompiled=false; profile events сохраняют hardwareUnitVersionKnown=false.
Provenance worker и отдельная проверка сверяют hashes request/observer/event log.

## Изменения реализации

`Invoke-CJ1InstructionOracle.ps1 -ProfileStateProbe -TimerCounterMode bcd`
принимает только plain NOP cases с полями id/mnemonic/operands. Дополнительные
поля, другая инструкция, binary mode и совмещение с другим probe отвергаются
до создания output directory и запуска worker. Frida observer тоже проверяет
ограничения. Другие режимы oracle сохраняют прежнее поведение.

`NativeProfileCandidate.LibraryState` теперь содержит отдельный
NativeLibraryProfileState: defaultVersionBytes/currentVersionBytes как числовые
массивы из3 bytes, extendedProtect, origin, sourceEventLogSHA256 и
hardwareUnitVersionKnown=false. Origin этого вложенного наблюдения и hash
журнала отделены от прежнего mask-only наблюдения. Никакая vendor DLL при
обычном Go-аудите не загружается; это исторические проверенные данные.

Настройка в текущем проекте или фактическое состояние CPU в момент запуска
аудита из этой структуры не выводятся. Изменение source CS2SupportVersion
на0/768/65535 оставляет эти retained значения прежними, а source-version-provenance
неподтверждённым. Возвращаемые экземпляры независимы. UnitVersionKnown,
FBSupportProfileVerified, SourceSettingsVerified, HardwareQualified,
WriteQualified и NetworkAccess не повышаются.

## Связь с будущей загрузкой

При таких локальных values selector &0x20 ==0 и IsExtendedProtect=0.
Это согласуется с ранее прослеженной обычной48-byte OBJ ветвью и условием
standard service3/0307. Это сопоставление локального контекста со статическим
предикатом; UI-object конкретной CX-One сессии и состояние настоящего ПЛК
им не установлены. Выбор0317/0307 в production sender не менялся.

Открыты actual target/unit version, все пути изменения/переноса защиты в
рабочем download context и CPU finalization служебных слов0/1/4, включая
различие real64/simulator256 и включение/исключение24-byte prefix. Опытов
записи UM не было. Для первой ограниченной загрузки остаётся конкретная
согласованная процедура candidate/readback/original return на изолированном
стенде; автоматические retry/force/снятие защиты не добавлялись.

## Проверки и сохранённые пакеты

После переноса на C: все10 пакетов Omron, Omron CLI и go vet ./omron/...
прошли. Отдельный offline replay дал27 проверок: точные6 событий и их
порядок, hashes библиотеки/request/observer/journal, значения двух профилей,
результаты NOP, отсутствие stderr, четыре PE byte anchors. Все source snapshots
совпали с реализацией побайтно. Четыре отрицательных launcher controls не
создали output directory и не запустили worker. Регрессия Go проверяет оба
профиля, три source stamp варианта и независимость возвращаемого состояния.

Frozen пакеты расположены под `C:/cx-proger/research`:

| Directory | Files excluding SHA256SUMS | Manifest SHA256 |
| --- | ---: | --- |
| hardware/20261119-cj1-local-profile-state-01 | 17 | `6db5b5766133f9f5e30f1f7a6cc720ab108b35bbba4783f898f85c698bf31693` |
| cj1-oracle/profile-state-20261119-01 | 11 | `7d02a1906220df770f9849f58b69c4569f1c35fe4e11bbfb1783ed0605f20943` |

Оригинальные F: пути внутри capture сохранены. Для offline replay задаются
актуальные параметры --repo, --capture, --binary и --output скрипту
verify-profile-state.py из hardware-пакета. Уже существующий отчёт проверяется,
не перезаписывается. Для Go-аудита/тестов достаточно Git и Go; Windows/DLL/Frida
нужны только для нового повторения исходного native capture.

Craft Anything MCP в этой сессии отвечает; его новая сессия находится в
Clean room, без открытого target. Для данного вывода использовались сохранённые
материалы и локальный verifier, а не новый анализ через MCP. Это не аппаратное
подтверждение профиля и не новый опыт загрузки.


Продолжение: [локальный current3 и predicates](omron-cj1-version3-library-profile.md)
измеряет явный SetCurrentVersion3.0 и reset в fresh KBase для каждой модели.
Предыдущая таблица after-SetPLC4.0 сохранена как исходное default-наблюдение;
она не заменяет подтверждённую через ПЛК и UI Unit Version3.0.
