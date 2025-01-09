create table evals (
  id text primary key default ('e_' || lower(hex(randomblob(16)))),
  created text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  updated text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  experiment text not null,
  name text not null,
  input text not null,
  expected text not null,
  output text not null,
  type text not null,
  score real not null,
  duration int not null
) strict;

create trigger evals_updated_timestamp after update on evals begin
  update evals set updated = strftime('%Y-%m-%dT%H:%M:%fZ') where id = old.id;
end;

create index evals_experiment on evals (experiment);
