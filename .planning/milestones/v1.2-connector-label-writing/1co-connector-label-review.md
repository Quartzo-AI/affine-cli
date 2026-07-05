# 1CO Connector Label Review

Status: applied after operator approval on 2026-06-20.

Plan file: `1co-connector-label-plan.review.json`
Plan ID: `canvas-labels-0674943e`
Workspace: `727cc066-a25e-4560-b68d-414b67cbc5c8`
Doc: `B6pvUw-r5SSfWKam-wncU`

## Labels

| From | To | Connector ID | Label |
|---|---|---|---|
| 1-Click-Outbound Sistema Completo | Lead Sources | `c000iaeier` | Mapa 1CO Para Fontes De Leads |
| Lead Sources | Merge And Dedup | `c0015koefu` | Lotes Brutos Com Origem Preservada |
| Merge And Dedup | Global Pool | `c0007ygitw` | Lead Consolidado E Deduplicado |
| Global Pool | Client Allocation | `c000ql2gto` | Disponibilidade Global Para Alocação |
| Client Allocation | Client Tables | `c000jaa9tv` | Alocação Por ICP E Campanha |
| Client Tables | Client Router | `c001tkcscq` | Contexto Operacional Do Cliente |
| Client Router | n8n Workflows | `c001fimc59` | Configuração Resolvida Sem Chave Exposta |
| n8n Workflows | Sales Engine | `c000x6y1cs` | Enriquecimento E Personalização |
| Sales Engine | Contact Channels | `c00133zw6e` | Personalização Comercial Do Toque |
| Contact Channels | Reply Engine | `c001rzi6tf` | Respostas E Sinais De Canal |
| Reply Engine | Quartzo Inbox | `c000rqsr4i` | Intenção Classificada E Draft |
| Quartzo Inbox | Quartzo CRM | `c000j87sqc` | Conversa Qualificada Para Pipeline |
| Client Tables | Pool Sync | `c000iatjd1` | Evento Local Com Quartzo-ID |
| Pool Sync | Global Pool | `c001gl59c8` | Status Global Sincronizado |
| Client Tables | Pool Reconciliation | `TS1c3K3Jpm` | Base Local Para Reconciliação |
| Pool Reconciliation | Global Pool | `c001j7xblq` | Correção De Drift E Cooldown |
| CRM Cross-Reference | Client Tables | `c001ku47qk` | Staging De Leads Do Inbox |
| CRM Cross-Reference | Quartzo CRM | `c0014ktb6b` | Contexto Cruzado De Oportunidade |
| Mutation Gates | n8n Workflows | `c000th5p53` | Aprovação Para Workflow Vivo |
| Mutation Gates | Contact Channels | `c000v49t2f` | Aprovação Para Envio Real |
| Mutation Gates | Quartzo CRM | `c001wcrpah` | Aprovação Para Registro Formal |
| Ops Surfaces | Client Router | `c000e36ji4` | Roteamento Operacional Por Cliente |
| Ops Surfaces | n8n Workflows | `c001113z7k` | Operação E Diagnóstico De Workflows |

## Verification

- `canvas apply --dry-run` accepted the plan and returned `live_write_supported: true`.
- Live write still requires `--live`, `--workspace`, `--doc`, `--backup-dir`, and `--yes`.
- `canvas doc integrity` returned `ok: true` and `issue_count: 0`.
- Live apply returned `applied: true`.
- Post-apply `canvas doc integrity` returned `ok: true`, `block_count: 1308`, and `issue_count: 0`.
- Post-apply live search matched all 23 planned labels with 0 mismatches.
- Backup artifacts written under `D:\Apps\QTZ-Apps\qtz-showrunner\00-brain\outputs\affine\backups`: `before.bin`, `before.b64`, `delta.bin`, `delta.b64`.

## Apply Command After Approval

Applied command:

```powershell
go run ./cmd/affine-pp-cli canvas apply --plan ..\..\.planning\milestones\v1.2-connector-label-writing\1co-connector-label-plan.review.json --live --workspace 727cc066-a25e-4560-b68d-414b67cbc5c8 --doc B6pvUw-r5SSfWKam-wncU --backup-dir D:\Apps\QTZ-Apps\qtz-showrunner\00-brain\outputs\affine\backups --yes --json
```

## Not In Current Board

These earlier proposed relations were not present as live connector edges, so they are not in the apply plan:

- Sales Engine -> n8n Workflows
- n8n Workflows -> Contact Channels
- Source Of Truth Boundary -> Global Pool
- Source Of Truth Boundary -> Quartzo CRM
