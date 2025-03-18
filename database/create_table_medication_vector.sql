use health ;


CREATE TABLE pathologies (
    id INT AUTO_INCREMENT PRIMARY KEY,
    nom VARCHAR(255) UNIQUE,
    embedding VECTOR(768) 
) TABLESPACE health_ts;

CREATE TABLE medicationv (
    id INT AUTO_INCREMENT PRIMARY KEY,
    nom VARCHAR(255),
    description TEXT,
    pathologie_id INT,
    embedding VECTOR(768),
    FOREIGN KEY (pathologie_id) REFERENCES pathologies(id)
) TABLESPACE health_ts;
