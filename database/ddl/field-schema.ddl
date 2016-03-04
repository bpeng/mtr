CREATE SCHEMA field;

CREATE TABLE field.model (
	modelPK SMALLSERIAL PRIMARY KEY,
	modelID TEXT NOT NULL UNIQUE
);

CREATE TABLE field.device (
	devicePK SMALLSERIAL PRIMARY KEY,
	deviceID TEXT NOT NULL UNIQUE,
	modelPK SMALLINT REFERENCES field.model(modelPK) ON DELETE CASCADE NOT NULL,
	latitude              NUMERIC(8,5) NOT NULL,
    	longitude             NUMERIC(8,5) NOT NULL,
	geom GEOGRAPHY(POINT, 4326) NOT NULL -- added via device_geom_trigger
);

CREATE FUNCTION field.device_geom() 
RETURNS  TRIGGER AS 
$$
BEGIN 
NEW.geom = ST_GeogFromWKB(st_AsEWKB(st_setsrid(st_makepoint(NEW.longitude, NEW.latitude), 4326)));
RETURN NEW;  END; 
$$
LANGUAGE plpgsql;

CREATE TRIGGER device_geom_trigger BEFORE INSERT OR UPDATE ON field.device
  FOR EACH ROW EXECUTE PROCEDURE field.device_geom();

CREATE TABLE field.type (
	typePK SMALLINT PRIMARY KEY,
	typeID TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL,
	unit TEXT NOT NULL
);

--  These must also be added to mtr-api/field_type.go
INSERT INTO field.type(typePK, typeID, description, unit) VALUES(1, 'voltage', 'voltage', 'mV'); 
INSERT INTO field.type(typePK, typeID, description, unit) VALUES(2, 'clock', 'clock quality', 'c'); 
INSERT INTO field.type(typePK, typeID, description, unit) VALUES(3, 'satellites', 'number of statellites tracked', 'n'); 
INSERT INTO field.type(typePK, typeID, description, unit) VALUES(4, 'conn', 'end to end connectivity', 'us'); 

CREATE TABLE field.metric_minute (
	devicePK SMALLINT REFERENCES field.device(devicePK) ON DELETE CASCADE NOT NULL,
	typePK SMALLINT REFERENCES field.type(typePK) ON DELETE CASCADE NOT NULL, 
	time TIMESTAMP(0) WITH TIME ZONE NOT NULL,
	min INTEGER NOT NULL,
	max INTEGER NOT NULL,
	PRIMARY KEY(devicePK, typePK, time)
);

CREATE INDEX on field.metric_minute (devicePK);
CREATE INDEX on field.metric_minute (typePK);

CREATE TABLE field.metric_hour (
	devicePK SMALLINT REFERENCES field.device(devicePK) ON DELETE CASCADE NOT NULL,
	typePK SMALLINT REFERENCES field.type(typePK) ON DELETE CASCADE NOT NULL, 
	time TIMESTAMP(0) WITH TIME ZONE NOT NULL,
	min INTEGER NOT NULL,
	max INTEGER NOT NULL,
	PRIMARY KEY(devicePK, typePK, time)
);

CREATE INDEX on field.metric_hour (devicePK);
CREATE INDEX on field.metric_hour (typePK);

CREATE TABLE field.metric_day (
	devicePK SMALLINT REFERENCES field.device(devicePK) ON DELETE CASCADE NOT NULL,
	typePK SMALLINT REFERENCES field.type(typePK) ON DELETE CASCADE NOT NULL, 
	time TIMESTAMP(0) WITH TIME ZONE NOT NULL,
	min INTEGER NOT NULL,
	max INTEGER NOT NULL,
	PRIMARY KEY(devicePK, typePK, time)
);

CREATE INDEX on field.metric_day (devicePK);
CREATE INDEX on field.metric_day (typePK);

CREATE TABLE field.metric_latest (
	devicePK SMALLINT REFERENCES field.device(devicePK) ON DELETE CASCADE NOT NULL,
	typePK SMALLINT REFERENCES field.type(typePK) ON DELETE CASCADE NOT NULL, 
	time TIMESTAMP(0) WITH TIME ZONE NOT NULL,
	value INTEGER NOT NULL,
	PRIMARY KEY(devicePK, typePK)
);

CREATE TABLE field.threshold (
	devicePK SMALLINT REFERENCES field.device(devicePK) ON DELETE CASCADE NOT NULL,
	typePK SMALLINT REFERENCES field.type(typePK) ON DELETE CASCADE NOT NULL, 
	lower INTEGER NOT NULL,
	upper INTEGER NOT NULL,
	PRIMARY KEY(devicePK, typePK)
);

CREATE INDEX on field.threshold (devicePK);
CREATE INDEX on field.threshold (typePK);

CREATE TABLE field.tag (
	tagPK SERIAL PRIMARY KEY,
	tag TEXT NOT NULL UNIQUE
);

CREATE TABLE field.metric_tag(
	devicePK SMALLINT REFERENCES field.device(devicePK) ON DELETE CASCADE NOT NULL,
	typePK SMALLINT REFERENCES field.type(typePK) ON DELETE CASCADE NOT NULL, 
	tagPK INTEGER REFERENCES field.tag(tagPK)  ON DELETE CASCADE NOT NULL,
	PRIMARY KEY(devicePK, typePK, tagPK)
);

CREATE INDEX on field.metric_tag (devicePK);
CREATE INDEX on field.metric_tag (typePK);
CREATE INDEX on field.metric_tag (tagPK);
