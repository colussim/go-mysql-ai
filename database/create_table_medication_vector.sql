use health ;

drop table medicationv;
drop table pathologies;

CREATE TABLE pathologies (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) UNIQUE,
    embedding VECTOR(896) 
) TABLESPACE health_ts;




CREATE TABLE medicationv (
    id INT AUTO_INCREMENT PRIMARY KEY,
    pathologie_id INT,
    drug_name VARCHAR(255),
    inactive_ingredient TEXT,
    purpose TEXT,
    keep_out_of_reach_of_children TEXT,
    warnings TEXT,
    spl_product_data_elements TEXT,
    dosage_and_administration TEXT,
    pregnancy_or_breast_feeding TEXT,
    package_label_principal_display_panel TEXT,
    indications_and_usage TEXT,
    embedding VECTOR(896),  
    CONSTRAINT fk_pathologie FOREIGN KEY (pathologie_id) REFERENCES pathologies(id)
) TABLESPACE health_ts;

