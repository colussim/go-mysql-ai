use health ;

CREATE TABLE medication (
    id INT AUTO_INCREMENT PRIMARY KEY,
    effective_time VARCHAR(20),
    purpose TEXT,
    keep_out_of_reach_of_children TEXT,
    when_using TEXT,
    questions TEXT,
    pregnancy_or_breast_feeding TEXT,
    storage_and_handling TEXT,
    indications_and_usage TEXT,
    set_id VARCHAR(255),
    ask_doctor_or_pharmacist TEXT,
    active_ingredient TEXT,
    dosage_and_administration TEXT,
    inactive_ingredient TEXT,
    warnings TEXT,
    version VARCHAR(10),
    package_label TEXT
) TABLESPACE health_ts;
