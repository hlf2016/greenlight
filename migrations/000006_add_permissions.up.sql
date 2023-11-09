CREATE TABLE IF NOT EXISTS permissions (
    id bigserial PRIMARY KEY ,
    code text NOT NULL
);

-- PRIMARY KEY (user_id, permission_id) 行在 users_permissions 表中设置了一个复合主键，其中主键由 users_id 和 permission_id 两列组成。
-- 将其设置为主键意味着，同一用户/权限组合在表中只能出现一次，不能重复出现
-- 在创建 users_permissions 表时，我们使用 REFERENCES user 语法针对 users 表的主键创建外键约束，确保 user_id 列中的任何值在 users 表中都有对应的条目。
-- 同样，我们使用 REFERENCES permissions 语法确保 permission_id 列在 permissions 表中有对应的条目。
CREATE TABLE  IF NOT EXISTS users_permissions (
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE ,
    permission_id bigint NOT NULL REFERENCES permissions ON DELETE CASCADE ,
    PRIMARY KEY (user_id, permission_id)
);

-- Add the two permissions to the table.
INSERT INTO permissions (code)
VALUES
    ('movies:read'),
    ('movies:write');