# postgres_exporter, version main (branch: main, revision: 22fb54d) → postgres_exporter, version v0.20.1 (branch: sync/v0.20.1, revision: 60e4a17)

| PG | node | endpoint | series old → new | names lost | names added | series lost | scrape old → new |
|---|---|---|---|---|---|---|---|
| pg13 | primary | default | 655 → 683 | 2 | 8 | 245 | 0.139925 → 0.100419 |
| pg13 | primary | hr | 607 → 635 | 2 | 8 | 245 | 0.060260 → 0.062793 |
| pg13 | primary | mr | 11 → 11 | 0 | 0 | 0 | 0.019769 → 0.018737 |
| pg13 | primary | lr | 36 → 36 | 0 | 0 | 0 | 0.026458 → 0.026115 |
| pg13 | replica | default | 644 → 668 | 1 | 3 | 240 | 0.101977 → 0.100783 |
| pg13 | replica | hr | 596 → 620 | 1 | 3 | 240 | 0.057093 → 0.061406 |
| pg13 | replica | mr | 11 → 11 | 0 | 0 | 0 | 0.019298 → 0.018868 |
| pg13 | replica | lr | 36 → 36 | 0 | 0 | 0 | 0.026821 → 0.027463 |
| pg14 | primary | default | 668 → 703 | 2 | 9 | 256 | 0.133769 → 0.132660 |
| pg14 | primary | hr | 620 → 655 | 2 | 9 | 256 | 0.077136 → 0.078242 |
| pg14 | primary | mr | 11 → 11 | 0 | 0 | 0 | 0.029500 → 0.026377 |
| pg14 | primary | lr | 36 → 36 | 0 | 0 | 0 | 0.036743 → 0.036108 |
| pg14 | replica | default | 657 → 687 | 1 | 4 | 251 | 0.132263 → 0.134726 |
| pg14 | replica | hr | 609 → 639 | 1 | 4 | 251 | 0.078836 → 0.076640 |
| pg14 | replica | mr | 11 → 11 | 0 | 0 | 0 | 0.029233 → 0.030558 |
| pg14 | replica | lr | 36 → 36 | 0 | 0 | 0 | 0.036222 → 0.037502 |
| pg15 | primary | default | 673 → 709 | 2 | 9 | 260 | 0.134721 → 0.132813 |
| pg15 | primary | hr | 625 → 660 | 2 | 9 | 260 | 0.075937 → 0.079032 |
| pg15 | primary | mr | 11 → 11 | 0 | 0 | 0 | 0.031042 → 0.029268 |
| pg15 | primary | lr | 36 → 36 | 0 | 0 | 0 | 0.040506 → 0.036353 |
| pg15 | replica | default | 662 → 694 | 1 | 4 | 255 | 0.184785 → 0.133338 |
| pg15 | replica | hr | 614 → 646 | 1 | 4 | 255 | 0.075538 → 0.080775 |
| pg15 | replica | mr | 11 → 11 | 0 | 0 | 0 | 0.028078 → 0.033970 |
| pg15 | replica | lr | 36 → 36 | 0 | 0 | 0 | 0.035890 → 0.037963 |
| pg16 | primary | default | 684 → 722 | 2 | 9 | 267 | 0.138040 → 0.150321 |
| pg16 | primary | hr | 636 → 674 | 2 | 9 | 267 | 0.078379 → 0.350845 |
| pg16 | primary | mr | 11 → 11 | 0 | 0 | 0 | 0.028606 → 0.037345 |
| pg16 | primary | lr | 36 → 36 | 0 | 0 | 0 | 0.035940 → 0.040829 |
| pg16 | replica | default | 673 → 706 | 1 | 4 | 262 | 0.136007 → 0.139928 |
| pg16 | replica | hr | 625 → 659 | 1 | 4 | 262 | 0.079974 → 0.087489 |
| pg16 | replica | mr | 11 → 11 | 0 | 0 | 0 | 0.029784 → 0.034572 |
| pg16 | replica | lr | 36 → 36 | 0 | 0 | 0 | 0.036004 → 0.043922 |
| pg17 | primary | default | 697 → 736 | 2 | 9 | 282 | 0.136658 → 0.139119 |
| pg17 | primary | hr | 649 → 688 | 2 | 9 | 282 | 0.078379 → 0.084067 |
| pg17 | primary | mr | 11 → 11 | 0 | 0 | 0 | 0.028548 → 0.029419 |
| pg17 | primary | lr | 36 → 36 | 0 | 0 | 0 | 0.036245 → 0.038441 |
| pg17 | replica | default | 686 → 720 | 1 | 4 | 277 | 0.129246 → 0.136142 |
| pg17 | replica | hr | 638 → 673 | 1 | 4 | 277 | 0.075646 → 0.094582 |
| pg17 | replica | mr | 11 → 11 | 0 | 0 | 0 | 0.027659 → 0.037959 |
| pg17 | replica | lr | 36 → 36 | 0 | 0 | 0 | 0.036809 → 0.040672 |
| pg18 | primary | default | 724 → 763 | 2 | 9 | 296 | 0.140686 → 0.142263 |
| pg18 | primary | hr | 676 → 716 | 2 | 9 | 296 | 0.080368 → 0.084538 |
| pg18 | primary | mr | 11 → 11 | 0 | 0 | 0 | 0.029559 → 0.029880 |
| pg18 | primary | lr | 36 → 36 | 0 | 0 | 0 | 0.038074 → 0.037628 |
| pg18 | replica | default | 713 → 749 | 1 | 4 | 291 | 0.139710 → 0.145026 |
| pg18 | replica | hr | 665 → 701 | 1 | 4 | 291 | 0.081047 → 0.085904 |
| pg18 | replica | mr | 11 → 11 | 0 | 0 | 0 | 0.030488 → 0.032669 |
| pg18 | replica | lr | 36 → 36 | 0 | 0 | 0 | 0.036546 → 0.039729 |

## Metric names lost (any version, node or endpoint)

         24 pg_stat_archiver_last_archive_age
         12 pg_stat_replication_reply_time

## Metric names added

         24 pg_roles_connection_limit
         24 pg_replication_last_replay_seconds
         24 pg_database_connection_limit
         20 pg_stat_database_active_time_seconds_total
         12 pg_replication_slots_wal_status
         12 pg_replication_slots_slot_is_active
         12 pg_replication_slots_slot_current_wal_lsn
         12 pg_replication_slots_slot_confirmed_flush_lsn
         12 pg_replication_slot_wal_status

## Series lost with label changes (same name, different labels), first 40

    pg_replication_slots_active{database="",server="127.0.0.1:5432",slot_name="baseline_slot"}
    pg_replication_slots_pg_wal_lsn_diff{database="",server="127.0.0.1:5432",slot_name="baseline_slot"}
    pg_settings_allow_in_place_tablespaces{server="127.0.0.1:5432"}
    pg_settings_allow_system_table_mods{server="127.0.0.1:5432"}
    pg_settings_archive_timeout_seconds{server="127.0.0.1:5432"}
    pg_settings_array_nulls{server="127.0.0.1:5432"}
    pg_settings_authentication_timeout_seconds{server="127.0.0.1:5432"}
    pg_settings_autovacuum_analyze_scale_factor{server="127.0.0.1:5432"}
    pg_settings_autovacuum_analyze_threshold{server="127.0.0.1:5432"}
    pg_settings_autovacuum_freeze_max_age{server="127.0.0.1:5432"}
    pg_settings_autovacuum_max_workers{server="127.0.0.1:5432"}
    pg_settings_autovacuum_multixact_freeze_max_age{server="127.0.0.1:5432"}
    pg_settings_autovacuum_naptime_seconds{server="127.0.0.1:5432"}
    pg_settings_autovacuum_vacuum_cost_delay_seconds{server="127.0.0.1:5432"}
    pg_settings_autovacuum_vacuum_cost_limit{server="127.0.0.1:5432"}
    pg_settings_autovacuum_vacuum_insert_scale_factor{server="127.0.0.1:5432"}
    pg_settings_autovacuum_vacuum_insert_threshold{server="127.0.0.1:5432"}
    pg_settings_autovacuum_vacuum_scale_factor{server="127.0.0.1:5432"}
    pg_settings_autovacuum_vacuum_threshold{server="127.0.0.1:5432"}
    pg_settings_autovacuum_work_mem_bytes{server="127.0.0.1:5432"}
    pg_settings_autovacuum{server="127.0.0.1:5432"}
    pg_settings_backend_flush_after_bytes{server="127.0.0.1:5432"}
    pg_settings_bgwriter_delay_seconds{server="127.0.0.1:5432"}
    pg_settings_bgwriter_flush_after_bytes{server="127.0.0.1:5432"}
    pg_settings_bgwriter_lru_maxpages{server="127.0.0.1:5432"}
    pg_settings_bgwriter_lru_multiplier{server="127.0.0.1:5432"}
    pg_settings_block_size{server="127.0.0.1:5432"}
    pg_settings_bonjour{server="127.0.0.1:5432"}
    pg_settings_check_function_bodies{server="127.0.0.1:5432"}
    pg_settings_checkpoint_completion_target{server="127.0.0.1:5432"}
    pg_settings_checkpoint_flush_after_bytes{server="127.0.0.1:5432"}
    pg_settings_checkpoint_timeout_seconds{server="127.0.0.1:5432"}
    pg_settings_checkpoint_warning_seconds{server="127.0.0.1:5432"}
    pg_settings_commit_delay{server="127.0.0.1:5432"}
    pg_settings_commit_siblings{server="127.0.0.1:5432"}
    pg_settings_cpu_index_tuple_cost{server="127.0.0.1:5432"}
    pg_settings_cpu_operator_cost{server="127.0.0.1:5432"}
    pg_settings_cpu_tuple_cost{server="127.0.0.1:5432"}
    pg_settings_cursor_tuple_fraction{server="127.0.0.1:5432"}
    pg_settings_data_checksums{server="127.0.0.1:5432"}
