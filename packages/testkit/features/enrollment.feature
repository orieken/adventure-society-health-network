Feature: Adventurer Enrollment
  Scenario: An Iron-rank adventurer enrolls in the Society
    Given an adventurer named "Farros" with rank "Iron" in guild "Grim Foundations"
    When they submit an enrollment request to the Adventure Society
    Then an 834 transaction is created with status "Accepted"
    And the adventurer's coverage status is "Active"

