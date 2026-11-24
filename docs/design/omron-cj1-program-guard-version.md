# CJ1: версия проверки целой программы — 2026-11-19

`PLC_Program::Compile` имеет два источника версии: current KBase и собственную
Program Check Version. В наблюдавшемся полном writer используется вторая ветвь.
Одной установки current3 недостаточно для квалификации version checks целого N1.
Добавлен отдельный исследовательский режим, который задаёт Program Check Version
именованным API и наблюдает фактические вызовы Compile.

## Уточнение предыдущих результатов

В [обычных instruction/image](omron-cj1-version3-compile.md) и
[FB/settings](omron-cj1-version3-fb.md) опытах current getter/fields действительно
были3.0/4.0, а файлы побайтно совпали с независимой реконструкцией. Эти измерения
сохраняются. Однако у собственных PLC_Program конструктор оставлял Program Check
Version FF/FF, и вызов Compile(...,false) выбирал её для version checks.
Прежние полные файлы нельзя считать самостоятельным подтверждением проверки
программы именно под3.0. Матрицы InstructionInst остаются отдельными наблюдениями;
обнаруженное правило относится к PLC_Program::Compile.

Новые опыты ниже явно устанавливают Program Check Version3.0/4.0. Старые frozen
captured events и manifests не исправлялись задним числом. Production
Profile.UnitVersion по-прежнему закрыт, поэтому неподтверждённой квалификации
в публичном кодеке не появилось.

## Статическая цепочка

Закреплённая CX-Pdata.dll SHA256
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`:

- Constructor121d5677 пишет FF в object+2bc/+2bd.
- SetProgramCheckVersion1221badb пишет два аргумента в эти поля.
- GetProgramCheckVersion1221bb00 читает их в две выходные byte-переменные.
- Compile121e3eef проверяет bool-аргумент по stack[EBP+c]. При true вызывает
  GetCurrentVersion121b0b4d; при false — GetProgramCheckVersion1221bb00.
  Пара упаковывается как major<<8|minor и попадает в descriptor+7c.

Getter/setter/constructor сняты свежим Craft disassembly. 104 байта ветвления
по121e4330 независимо сняты из оригинального PE и сопоставлены с прежним полным
Ghidra report `ghidra-instruction-compile.json.gz` (проверяется его SHA256).
FF/FF сохраняется как исходное значение; его общий смысл и все последствия
для проверок здесь не назначаются по предположению.

## Локальная матрица

Каждый из шести запусков использует два собственных проекта на двух моделях:
LD W0.00 / OUT W0.01 / END без FB и END с неиспользуемым ProbeFB-определением.
Timer BCD. Сначала явный current, затем при необходимости нормальный
SetProgramCheckVersion для task и definition, до их компиляции.

| KBase current | Program Check Version | Наблюдавшийся выбранный guard | Packed после Compile |
| --- | --- | --- | --- |
| 3.0 | constructor FF/FF | FF/FF | FFFF |
| 3.0 | 3.0 | 3,0 | 0300 |
| 3.0 | 4.0 | 4,0 | 0400 |
| 4.0 | constructor FF/FF | FF/FF | FFFF |
| 4.0 | 3.0 | 3,0 | 0300 |
| 4.0 | 4.0 | 4,0 | 0400 |

Получены36 регистраций объектов и60 вызовов Compile. Observer читает this/stack
при входе, getter и поля обеих версий, затем возвращённый результат, состояния
после и packed guard. Все наблюдавшиеся bool-аргументы — false. Несовпадающие
current/check варианты устанавливают выбор источника независимо от равенства
самих3.0/4.0. True-ветвь прослежена статически, но в runtime не выполнялась.

24 полных N1 побайтно одинаковы между вариантами и независимо восстановлены
Go helper: 12 файлов2632 bytes без FB и12 файлов2756 bytes с определением.
Task Compile возвращает1; definition Compile0/errors0/warnings1 с прежним
Unreferenced Function Block Body warning; full writer1/errorarray0. Это простые
проекты без version-sensitive новой инструкции. Равные файлы не доказывают
равенство всей палитры при3.0/4.0.

## Инструмент и переносимость

Новый необязательный параметр launcher:
`-ProgramCheckVersion constructor-default|3.0|4.0`. Требует ProgramObjectProbe,
явный LocalCurrentVersion и BCD. Source создаётся из прежнего pinned base и
current insertion, затем из отдельной `program-check-version-insert.js`.
Три новых anchors проверяются на единственность; сохраняются оба последовательных
transforms, их hashes и13 provenance files. Старый observer не менялся.

Hooks только наблюдают вызовы собственных объектов; они не подменяют arguments,
returns или flags. Явная версия задаётся native setter перед Compile. Worker
ограничен по числу объектов/вызовов и времени; GUI и сеть не используются.
Raw artifacts: `research/cj1-oracle/program-guard-{default,check30,check40}-current{30,40}-20261119-01`.
Static/raw source snapshots, logs и manifests:
`C:/cx-proger/research/hardware/20261119-cj1-referenced-fb-01`.

Portable corpus `omron/codec/testdata/native-program-guard-version.json.gz`
включает статические checkpoints и события. Go тест независимо восстанавливает
все24 файла и проверяет выбор guard. Python `verify_cj1_program_guard.py`
проверяет матрицу, raw/provenance, оба преобразования source и injected request.
Replay не загружает DLL и не требует Windows или ПЛК.

Все10 пакетов Omron, `go vet ./omron/...` и49 Python tests прошли. Полный raw
replay дал677 проверок. Четыре неверные комбинации параметров launcher отклонены
до создания output/worker. Общий cmd/naut suite повторно не запускался;
его ранее выявленные отдельные Windows failures остаются в предыдущем отчёте.

```text
python -B tools/omron-research/verify_cj1_program_guard.py --repo . --capture-root C:/cx-proger/research/cj1-oracle
go test ./omron/...
```

## Следующий gate

Следующий полный FB опыт должен явно задавать **обе** версии и сохранять эту
трассу при связывании definition, instance section и вызова из задачи.
Такой referenced FB image здесь ещё не создан; definition остаётся неиспользуемым.
Также остаются version-sensitive запреты, initialization/settings provenance,
production explicit profile и аппаратные transfer/finalization/restore gates.
Новых PLC-команд или записей UM не было; действий оператора для локального
продолжения сейчас не требуется.


Продолжение: [native binding и invocation boundary](omron-cj1-instance-binding.md)
подтверждает complete definition/instance/direct-call файлы с обеими версиями.
Полный параметрический caller пока отвергается native task checking.
